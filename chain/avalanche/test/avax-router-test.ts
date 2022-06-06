import { deployments, ethers, getNamedAccounts } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";


describe("AvaxRouter", function () {
    let accounts: SignerWithAddress[];
    let avaxRouter: any


    beforeEach(async () => {
        accounts = await ethers.getSigners();
        await deployments.fixture();
        const avaxRouterDeployment = await ethers.getContractFactory(
            "AvaxRouter"
        );
        avaxRouter = await avaxRouterDeployment.deploy()
    });

    describe("initialize", function () {
        it("Should init", async () => {
            expect(ethers.utils.isAddress(avaxRouter.address)).eq(true);
            expect(avaxRouter.address).to.not.eq(ethers.constants.AddressZero);
        });
    });

    describe("User Deposit Assets", function () {
        it("Should Deposit Ether To Asgard1", async function () {
        });
        it("Should revert Deposit Ether To Asgard1", async function () {

        });
        it("Should Deposit RUNE to Asgard1", async function () {
        })
        it("Should revert Deposit RUNE to Asgard1", async function () {
        })
        it("Should Deposit Token to Asgard1", async function () {
        })
        it("Should revert Deposit Token to Asgard1", async function () {
        })
        it("Should revert when ETH sent during ERC20 Deposit", async function () {
        })
        it("Should Deposit USDT to Asgard1", async function () {
        })

    });
    describe("Fund Yggdrasil, Yggdrasil Transfer Out", function () {

        it("Should fund yggdrasil ETH", async function () {

        });

        it("Should fund yggdrasil tokens", async function () {


        });

        it("Should transfer ETH to USER2", async function () {

        });

        it("Should take ETH amount from the amount in transaction, instead of the amount parameter", async function () {

        });

        it("Should transfer tokens to USER2", async function () {

        });
        it("Should transfer USDT to USER2", async function () {

        });

    });

    describe("Yggdrasil Returns Funds, Asgard Churns, Old Vaults can't spend", function () {

        it("Ygg returns", async function () {

        });
        it("Asgard Churns", async function () {

        });
        it("Should fail to when old Asgard interacts", async function () {

        });
        it("Should fail to when old Yggdrasil interacts", async function () {

        });
    });

})
