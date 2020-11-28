const expect = require('chai').expect;
const {By, until, error} =  require('selenium-webdriver');
const baseSteps = require(__srcdir + '/steps/baseSteps.js');
const signinPage = require(__srcdir + '/pages/signin/signinPage.js');
const influxPage = require(__srcdir + '/pages/influxPage.js');

class signinSteps extends baseSteps {

    constructor(driver){
        super(driver);
        this.signinPage = new signinPage(driver);
        this.influxPage = new influxPage(driver);
    }

    async verifyHeadingContains(text){
        expect(await (await this.signinPage.getHeading()).getText()).to.include(text);
    }

    async verifyHeading(){
        await this.assertVisible(await this.signinPage.getInfluxLogo());
    }

    async verifyVersionContains(version){
        expect(await (await this.signinPage.getVersionInfo()).getText()).to.include(version);
    }

    async verifyCreditsLink(){
        await this.signinPage.getCreditsLink().then(  elem => {
            elem.getText().then( eltxt => {
                expect(eltxt).to.equal('InfluxData');
            });

            elem.getAttribute('href').then(href => {
                expect(href).to.equal('https://www.influxdata.com/');
            });
        });
    }

    async verifyIsLoaded(){
        this.assertVisible(await this.signinPage.getInfluxLogo());
        this.assertVisible(await this.signinPage.getNameInput());
        this.assertVisible(await this.signinPage.getPasswordInput());
        this.assertVisible(await this.signinPage.getSigninButton());
        //this.assertVisible(await this.signinPage.getCreditsLink());
    }

    async enterUsername(name){
        await this.signinPage.getNameInput().then(async input => {
            await input.clear();
            await input.sendKeys(name);
        }).catch(async err => {
            console.error("ERROR on input user name: " + err);
            if(err instanceof error.StaleElementReferenceError){ // try again
                console.warn("WARNING retrying input username")
//                await this.driver.sleep(3000); //
                await this.driver.wait(until.elementIsVisible(await this.signinPage.getNameInput()));
                await this.signinPage.getNameInput().then(async input => {
                    await input.clear();
                    await input.sendKeys(name);
                })
            }
            throw err;
        });
    }

    async enterPassword(password){
        await this.signinPage.getPasswordInput().then(async input =>{
            await input.clear();
            await input.sendKeys(password);
        }).catch(async err => {
            console.log("ERROR on input user password: " + err);
            if(err instanceof error.StaleElementReferenceError) { // try again
                console.warn("WARNING retrying input password");
//                await this.driver.sleep(3000); //
                await this.driver.wait(until.elementIsVisible(await this.signinPage.getPasswordInput()));
                await this.signinPage.getPasswordInput().then(async input =>{
                    await input.clear();
                    await input.sendKeys(password);
                })
            }
            throw err;
        });
    }

    async clickSigninButton(){
        await this.signinPage.getSigninButton().then(async btn =>{
            await btn.click();
        }).catch(async err => {
            console.error("ERROR on click signin button: " + err);
            if(err instanceof error.StaleElementReferenceError) { //try again
                console.warn("WARNING retrying click signin");
//                await this.driver.sleep(3000); //
                await this.driver.wait(until.elementIsVisible(await this.signinPage.getSigninButton()));
                await this.signinPage.getSigninButton().then(async btn =>{
                    await btn.click();
                })
            }
            throw err;
        });
    }

    async waitForSigninToLoad(timeout){
        await this.driver.wait(until.elementIsVisible(await this.signinPage.getNameInput()), timeout,
            `Waited ${timeout} milliseconds to locate signin name input`);
        await this.driver.wait(until.elementIsVisible(await this.signinPage.getPasswordInput()), timeout,
            `Waited ${timeout} milliseconds to locate signin password input`);
    }

    async signin(user){
        await this.driver.sleep(5000); //work around for stale element when signin page refreshes
        await this.enterUsername(user.username);
        await this.enterPassword(user.password);
        await this.clickSigninButton();
        await this.influxPage.isLoaded();
    }
}

module.exports = signinSteps;

