import { deployments, ethers, getNamedAccounts, network } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";
import { BigNumber, Contract, ContractReceipt, Signer } from "ethers";
import { USDCE_ADDRESS, USDCE_WHALE } from "./constants";
import ERC20 from "@openzeppelin/contracts/build/contracts/ERC20.json";
import { AvaxRouter } from "../typechain-types";

describe("AvaxRouter", function () {
    let accounts: SignerWithAddress[];
    let avaxRouter: AvaxRouter
    let usdceToken: Contract
    const AVAX = ethers.constants.AddressZero

    beforeEach(async () => {
        const { wallet1 } = await getNamedAccounts();

        accounts = await ethers.getSigners();
        await deployments.fixture();
        const avaxRouterDeployment = await ethers.getContractFactory(
            "AvaxRouter"
        );
        avaxRouter = await avaxRouterDeployment.deploy()

        usdceToken = new ethers.Contract(
            USDCE_ADDRESS,
            ERC20.abi,
            accounts[0]
        );

        // Transfer UCDCE to admin
        const transferAmount = "150000000000"; // 6 dec

        await network.provider.request({
            method: "hardhat_impersonateAccount",
            params: [USDCE_WHALE],
        });


        const whaleSigner = await ethers.getSigner(USDCE_WHALE);
        const avaxRouterUSDCEWhale = usdceToken.connect(whaleSigner);
        await avaxRouterUSDCEWhale.transfer(wallet1, transferAmount);
        const usdceBalance = await avaxRouterUSDCEWhale.balanceOf(wallet1);
        expect(usdceBalance).eq(transferAmount)
    });

    describe("initialize", function () {
        it("Should init", async () => {
            expect(ethers.utils.isAddress(avaxRouter.address)).eq(true);
            expect(avaxRouter.address).to.not.eq(ethers.constants.AddressZero);
        });
    });

    describe("User Deposit Assets", function () {
        it("Should Deposit AVAX To Asgard", async function () {
            const { asgard1 } = await getNamedAccounts();
            const amount = ethers.utils.parseEther("1000")

            let startBal = BigNumber.from(await ethers.provider.getBalance(asgard1))
            let tx = await avaxRouter.deposit(asgard1, AVAX, amount,
                'SWAP:THOR.RUNE', { value: amount });
            const receipt = await tx.wait()

            expect(receipt?.events?.[0].event).to.equal('Deposit')
            expect(tx.value).to.equal(amount)
            expect(receipt?.events?.[0]?.args?.asset).to.equal(AVAX)
            expect(receipt?.events?.[0]?.args?.memo).to.equal("SWAP:THOR.RUNE")

            let endBal = BigNumber.from(await ethers.provider.getBalance(asgard1))
            let changeBal = BigNumber.from(endBal).sub(startBal)
            expect(changeBal).to.equal(amount);

        });
        it("Should revert expired Deposit AVAX To Asgard1", async function () {
            const { asgard1 } = await getNamedAccounts();
            const amount = ethers.utils.parseEther("1000")

            await expect(avaxRouter.depositWithExpiry(asgard1, AVAX, amount, "SWAP:THOR.RUNE:tthor1uuds8pd92qnnq0udw0rpg0szpgcslc9p8lluej",
                BigNumber.from(0), { value: amount })).to.be.revertedWith("THORChain_Router: expired")

        });
        it("Should Deposit Token to Asgard1", async function () {
            const { wallet1, asgard1 } = await getNamedAccounts();
            const amount = "500000000"

            const wallet1Signer = accounts.find(
                (account) => account.address === wallet1
            );

            // approve usdce transfer
            const avaxRouterWallet1 = avaxRouter.connect(wallet1Signer as Signer);
            const usdceTokenWallet1 = usdceToken.connect(wallet1Signer as Signer);
            await usdceTokenWallet1.approve(avaxRouterWallet1.address, amount);

            let tx = await avaxRouterWallet1.deposit(asgard1, usdceToken.address, amount,
                'SWAP:THOR.RUNE');
            const receipt = await tx.wait()

            receipt?.events?.find(event => {
                if (event.logIndex === 2) {
                    expect(event.event).to.equal('Deposit')
                    expect(event.args?.asset.toLowerCase()).to.equal(USDCE_ADDRESS)
                    expect(event.args?.to).to.equal(asgard1)
                    expect(event.args?.memo).to.equal("SWAP:THOR.RUNE")
                    expect(event.args?.amount).to.equal(amount)
                }
            })

            expect(await usdceToken.balanceOf(avaxRouter.address)).to.equal(amount);
            expect(await avaxRouterWallet1.vaultAllowance(asgard1, usdceToken.address)).to.equal(amount);

        })
        it("Should revert Deposit Token to Asgard1", async function () {
            const { asgard1 } = await getNamedAccounts();
            const amount = "500000000"

            await expect(avaxRouter.depositWithExpiry(asgard1, usdceToken.address, amount, "SWAP:THOR.RUNE:tthor1uuds8pd92qnnq0udw0rpg0szpgcslc9p8lluej",
                BigNumber.from(0))).to.be.revertedWith("THORChain_Router: expired")
        })
        it("Should revert when AVAX sent during ARC20 Deposit", async function () {
        })
        it("Should Deposit USDT to Asgard1", async function () {
        })

    });
    describe("Fund Yggdrasil, Yggdrasil Transfer Out", function () {

        it("Should fund yggdrasil AVAX", async function () {

        });

        it("Should fund yggdrasil tokens", async function () {


        });

        it("Should transfer AVAX to Wallet2", async function () {

        });

        it("Should take AVAX amount from the amount in transaction, instead of the amount parameter", async function () {

        });

        it("Should transfer tokens to Wallet2", async function () {

        });
        it("Should transfer USDT to Wallet2", async function () {

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
