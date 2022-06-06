import { deployments, ethers, getNamedAccounts } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";
import { BigNumber } from "ethers";


describe("AvaxRouter", function () {
    let accounts: SignerWithAddress[];
    let avaxRouter: any
    const AVAX = ethers.constants.AddressZero

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
        it("Should Deposit AVAX To Asgard", async function () {
            const { wallet1, asgard1 } = await getNamedAccounts();
            const amount = ethers.utils.parseEther("1000")
            const wallet1Signer = accounts.find(
                (account) => account.address === wallet1
            );
            const routerWallet1 = avaxRouter.connect(wallet1Signer);

            let startBal = BigNumber.from(await ethers.provider.getBalance(asgard1))
            let tx = await routerWallet1.deposit(asgard1, AVAX, amount,
                'SWAP:THOR.RUNE', { value: amount });
            const receipt = await tx.wait()

            expect(receipt.events[0].event).to.equal('Deposit')
            expect(tx.value).to.equal(amount)
            expect(receipt.events[0].args.asset).to.equal(AVAX)
            expect(receipt.events[0].args.memo).to.equal("SWAP:THOR.RUNE")

            let endBal = BigNumber.from(await ethers.provider.getBalance(asgard1))
            let changeBal = BigNumber.from(endBal).sub(startBal)
            expect(changeBal).to.equal(amount);

        });
        it("Should revert expired Deposit AVAX To Asgard1", async function () {
            const { wallet1, asgard1 } = await getNamedAccounts();
            const amount = ethers.utils.parseEther("1000")
            const wallet1Signer = accounts.find(
                (account) => account.address === wallet1
            );
            const routerWallet1 = avaxRouter.connect(wallet1Signer);

            await expect(routerWallet1.depositWithExpiry(asgard1, AVAX, amount, "SWITCH:THOR.RUNE",
                BigNumber.from(0), { value: amount })).to.be.revertedWith("THORChain_Router: expired") //, "THORChain_Router: expired");

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
