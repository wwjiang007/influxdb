package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/influxdata/influxdb/v2"
	"github.com/influxdata/influxdb/v2/bolt"
	"github.com/influxdata/influxdb/v2/http"
	influxlogger "github.com/influxdata/influxdb/v2/logger"
	"github.com/influxdata/influxdb/v2/tenant"
	"github.com/influxdata/influxdb/v2/v1/services/meta"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func cmdRestore(f *globalFlags, opts genericCLIOpts) *cobra.Command {
	return newCmdRestoreBuilder(f, opts).cmdRestore()
}

type cmdRestoreBuilder struct {
	genericCLIOpts
	*globalFlags

	full          bool
	bucketID      string
	bucketName    string
	newBucketName string
	newOrgName    string
	org           organization
	path          string

	kvEntry      *influxdb.ManifestKVEntry
	shardEntries map[uint64]*influxdb.ManifestEntry

	orgService     *tenant.OrgClientService
	bucketService  *tenant.BucketClientService
	restoreService *http.RestoreService
	tenantService  *tenant.Service
	metaClient     *meta.Client

	logger *zap.Logger
}

func newCmdRestoreBuilder(f *globalFlags, opts genericCLIOpts) *cmdRestoreBuilder {
	return &cmdRestoreBuilder{
		genericCLIOpts: opts,
		globalFlags:    f,

		shardEntries: make(map[uint64]*influxdb.ManifestEntry),
	}
}

func (b *cmdRestoreBuilder) cmdRestore() *cobra.Command {
	cmd := b.newCmd("restore", b.restoreRunE)
	b.org.register(b.viper, cmd, true)
	cmd.Flags().BoolVar(&b.full, "full", false, "Fully restore and replace all data on server")
	cmd.Flags().StringVar(&b.bucketID, "bucket-id", "", "The ID of the bucket to restore")
	cmd.Flags().StringVarP(&b.bucketName, "bucket", "b", "", "The name of the bucket to restore")
	cmd.Flags().StringVar(&b.newBucketName, "new-bucket", "", "The name of the bucket to restore to")
	cmd.Flags().StringVar(&b.newOrgName, "new-org", "", "The name of the organization to restore to")
	cmd.Flags().StringVar(&b.path, "input", "", "Local backup data path (required)")
	cmd.Use = "restore [flags] path"
	cmd.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("must specify path to backup directory")
		} else if len(args) > 1 {
			return fmt.Errorf("too many args specified")
		}
		b.path = args[0]
		return nil
	}
	cmd.Short = "Restores a backup directory to InfluxDB."
	cmd.Long = `
Restore influxdb.

Examples:
	# restore all data
	influx restore /path/to/restore
`
	return cmd
}

func (b *cmdRestoreBuilder) restoreRunE(cmd *cobra.Command, args []string) (err error) {
	ctx := context.Background()

	// Create top level logger
	logconf := influxlogger.NewConfig()
	if b.logger, err = logconf.New(os.Stdout); err != nil {
		return err
	}

	// Ensure org/bucket filters are set if a new org/bucket name is specified.
	if b.newOrgName != "" && b.org.id == "" && b.org.name == "" {
		return fmt.Errorf("must specify source org id or name when renaming restored org")
	} else if b.newBucketName != "" && b.bucketID == "" && b.bucketName == "" {
		return fmt.Errorf("must specify source bucket id or name when renaming restored bucket")
	}

	// Read in set of KV data & shard data to restore.
	if err := b.loadIncremental(); err != nil {
		return fmt.Errorf("restore failed while processing manifest files: %s", err.Error())
	} else if b.kvEntry == nil {
		return fmt.Errorf("no manifest files found in: %s", b.path)
	}

	ac := flags.config()
	b.restoreService = &http.RestoreService{
		Addr:               ac.Host,
		Token:              ac.Token,
		InsecureSkipVerify: flags.skipVerify,
	}

	client, err := newHTTPClient()
	if err != nil {
		return err
	}

	b.orgService = &tenant.OrgClientService{Client: client}
	b.bucketService = &tenant.BucketClientService{Client: client}

	if !b.full {
		return b.restorePartial(ctx)
	}
	return b.restoreFull(ctx)
}

// restoreFull completely replaces the bolt metadata file and restores all shard data.
func (b *cmdRestoreBuilder) restoreFull(ctx context.Context) (err error) {
	if err := b.restoreKVStore(ctx); err != nil {
		return err
	}

	// Restore each shard for the bucket.
	for _, file := range b.shardEntries {
		if err := b.restoreShard(ctx, file.ShardID, file); err != nil {
			return err
		}
	}

	return nil
}

func (b *cmdRestoreBuilder) restoreKVStore(ctx context.Context) (err error) {
	f, err := os.Open(filepath.Join(b.path, b.kvEntry.FileName))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := b.restoreService.RestoreKVStore(ctx, f); err != nil {
		return err
	}
	b.logger.Info("Full metadata restored.")

	return nil
}

// restorePartial restores shard data to a server without deleting existing data.
// Organizations & buckets are created as needed. Cannot overwrite an existing bucket.
func (b *cmdRestoreBuilder) restorePartial(ctx context.Context) (err error) {
	// Open bolt DB.
	boltClient := bolt.NewClient(b.logger)
	boltClient.Path = filepath.Join(b.path, b.kvEntry.FileName)
	if err := boltClient.Open(ctx); err != nil {
		return err
	}
	defer boltClient.Close()

	// Open meta store so we can iterate over meta data.
	kvStore := bolt.NewKVStore(b.logger, boltClient.Path)
	kvStore.WithDB(boltClient.DB())

	tenantStore := tenant.NewStore(kvStore)
	b.tenantService = tenant.NewService(tenantStore)

	b.metaClient = meta.NewClient(meta.NewConfig(), kvStore)
	if err := b.metaClient.Open(); err != nil {
		return err
	}

	// Filter through organizations & buckets to restore appropriate shards.
	if err := b.restoreOrganizations(ctx); err != nil {
		return err
	}

	b.logger.Info("Restore complete")

	return nil
}

func (b *cmdRestoreBuilder) restoreOrganizations(ctx context.Context) (err error) {
	// Build a filter if org ID or org name were specified.
	var filter influxdb.OrganizationFilter
	if b.org.id != "" {
		if filter.ID, err = influxdb.IDFromString(b.org.id); err != nil {
			return err
		}
	} else if b.org.name != "" {
		filter.Name = &b.org.name
	}

	// Retrieve a list of all matching organizations.
	orgs, _, err := b.tenantService.FindOrganizations(ctx, filter)
	if err != nil {
		return err
	}

	// Restore matching organizations.
	for _, org := range orgs {
		if err := b.restoreOrganization(ctx, org); err != nil {
			return err
		}
	}
	return nil
}

func (b *cmdRestoreBuilder) restoreOrganization(ctx context.Context, org *influxdb.Organization) (err error) {
	b.logger.Info("Restoring organization", zap.String("id", org.ID.String()), zap.String("name", org.Name))

	newOrg := *org
	if b.newOrgName != "" {
		newOrg.Name = b.newOrgName
	}

	// Create organization on server, if it doesn't already exist.
	if o, err := b.orgService.FindOrganization(ctx, influxdb.OrganizationFilter{Name: &newOrg.Name}); influxdb.ErrorCode(err) == influxdb.ENotFound {
		if err := b.orgService.CreateOrganization(ctx, &newOrg); err != nil {
			return fmt.Errorf("cannot create organization: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("cannot find existing organization: %#v", err)
	} else {
		newOrg.ID = o.ID
	}

	// Build a filter if bucket ID or bucket name were specified.
	var filter influxdb.BucketFilter
	filter.OrganizationID = &org.ID // match on backup's org ID
	if b.bucketID != "" {
		if filter.ID, err = influxdb.IDFromString(b.bucketID); err != nil {
			return err
		}
	} else if b.bucketName != "" {
		filter.Name = &b.bucketName
	}

	// Retrieve a list of all buckets for the organization in the local backup.
	buckets, _, err := b.tenantService.FindBuckets(ctx, filter)
	if err != nil {
		return err
	}

	// Restore each matching bucket.
	for _, bkt := range buckets {
		// Skip internal buckets.
		if strings.HasPrefix(bkt.Name, "_") {
			continue
		}

		bkt = bkt.Clone()
		bkt.OrgID = newOrg.ID

		if err := b.restoreBucket(ctx, bkt); err != nil {
			return err
		}
	}
	return nil
}

func (b *cmdRestoreBuilder) restoreBucket(ctx context.Context, bkt *influxdb.Bucket) (err error) {
	b.logger.Info("Restoring bucket", zap.String("id", bkt.ID.String()), zap.String("name", bkt.Name))

	// Create bucket on server.
	newBucket := *bkt
	if b.newBucketName != "" {
		newBucket.Name = b.newBucketName
	}
	if err := b.bucketService.CreateBucket(ctx, &newBucket); err != nil {
		return fmt.Errorf("cannot create bucket: %w", err)
	}

	// Lookup matching database from the meta store.
	// Search using bucket ID from backup.
	dbi := b.metaClient.Database(bkt.ID.String())
	if dbi == nil {
		return fmt.Errorf("bucket database not found: %s", bkt.ID.String())
	}

	// Serialize to protobufs.
	buf, err := dbi.MarshalBinary()
	if err != nil {
		return fmt.Errorf("cannot marshal database info: %w", err)
	}

	shardIDMap, err := b.restoreService.RestoreBucket(ctx, newBucket.ID, buf)
	if err != nil {
		return fmt.Errorf("cannot restore bucket: %w", err)
	}

	// Restore each shard for the bucket.
	for _, file := range b.shardEntries {
		if bkt.ID.String() != file.BucketID {
			continue
		}

		// Skip if shard metadata was not imported.
		newID, ok := shardIDMap[file.ShardID]
		if !ok {
			b.logger.Warn("Meta info not found, skipping file", zap.Uint64("shard", file.ShardID), zap.String("bucket_id", file.BucketID), zap.String("filename", file.FileName))
			return nil
		}

		if err := b.restoreShard(ctx, newID, file); err != nil {
			return err
		}
	}

	return nil
}

func (b *cmdRestoreBuilder) restoreShard(ctx context.Context, newShardID uint64, file *influxdb.ManifestEntry) error {
	b.logger.Info("Restoring shard live from backup", zap.Uint64("shard", newShardID), zap.String("filename", file.FileName))

	f, err := os.Open(filepath.Join(b.path, file.FileName))
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	return b.restoreService.RestoreShard(ctx, newShardID, gr)
}

// loadIncremental loads multiple manifest files from a given directory.
func (b *cmdRestoreBuilder) loadIncremental() error {
	// Read all manifest files from path, sort in descending time.
	manifests, err := filepath.Glob(filepath.Join(b.path, "*.manifest"))
	if err != nil {
		return err
	} else if len(manifests) == 0 {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(manifests)))

	b.shardEntries = make(map[uint64]*influxdb.ManifestEntry)
	for _, filename := range manifests {
		// Skip file if it is a directory.
		if fi, err := os.Stat(filename); err != nil {
			return err
		} else if fi.IsDir() {
			continue
		}

		// Read manifest file for backup.
		var manifest influxdb.Manifest
		if buf, err := ioutil.ReadFile(filename); err != nil {
			return err
		} else if err := json.Unmarshal(buf, &manifest); err != nil {
			return fmt.Errorf("read manifest: %v", err)
		}

		// Save latest KV entry.
		if b.kvEntry == nil {
			b.kvEntry = &manifest.KV
		}

		// Load most recent backup per shard.
		for i := range manifest.Files {
			sh := manifest.Files[i]
			if _, err := os.Stat(filepath.Join(b.path, sh.FileName)); err != nil {
				continue
			}

			entry := b.shardEntries[sh.ShardID]
			if entry == nil || sh.LastModified.After(entry.LastModified) {
				b.shardEntries[sh.ShardID] = &sh
			}
		}
	}

	return nil
}

func (b *cmdRestoreBuilder) newCmd(use string, runE func(*cobra.Command, []string) error) *cobra.Command {
	cmd := b.genericCLIOpts.newCmd(use, runE, true)
	b.genericCLIOpts.registerPrintOptions(cmd)
	b.globalFlags.registerFlags(b.viper, cmd)
	return cmd
}
