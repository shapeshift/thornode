import { ethers, getNamedAccounts } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";
import { BigNumber, Signer } from "ethers";
import { pangolinRouterAbi } from "../src/abis/pangolinRouterAbi";

describe("PangolinRouter", function () {
  let accounts: SignerWithAddress[];
  let pangolin: any; // TODO: fix
  const pangolinRouter = "0xE54Ca86531e17Ef3616d22Ca28b0D458b6C89106";
  const wAvaxAddress = "0xB31f66AA3C1e785363F0875A1B74E27b85FD66c7";
  const usdceAddress = "0xa7d7079b0fead91f3e65f86e8915cb59c1a4c664";

  beforeEach(async () => {
    accounts = await ethers.getSigners();
    pangolin = new ethers.Contract(
      pangolinRouter,
      pangolinRouterAbi,
      accounts[0]
    );
  });

  describe("initialize", function () {
    it("Should init", async () => {
      expect(ethers.utils.isAddress(pangolin.address)).eq(true);
      expect(pangolin.address).to.not.eq(ethers.constants.AddressZero);
    });
    it.skip("Has AVAX", async () => {});
  });

  describe("swap", function () {
    it("Should swap Avax for USDC.e", async () => {
      const { wallet1 } = await getNamedAccounts();

      const amountOutMin = "2000000";

      const wallet1Signer = accounts.find(
        (account) => account.address === wallet1
      );
      const pangolinWallet1 = pangolin.connect(wallet1Signer as Signer);
      const currentBlock = await ethers.provider.getBlockNumber();
      const currentTime = (await ethers.provider.getBlock(currentBlock))
        .timestamp;

      const estimatedTransferAmount =
        await pangolinWallet1.swapExactAVAXForTokens(
          amountOutMin,
          [wAvaxAddress, usdceAddress],
          wallet1,
          currentTime + 1000000000,
          { value: ethers.utils.parseEther("0.1") }
        );

      const usdceContract = await ethers.getContractAt("IERC20", usdceAddress);
      const balanceOfUsdce = await usdceContract.balanceOf(wallet1);
      expect(balanceOfUsdce).not.eq(0)
    });
  });
});
