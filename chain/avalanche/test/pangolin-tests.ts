import { ethers, getNamedAccounts } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";
import { Signer } from "ethers";
import { pangolinRouterAbi } from "../src/abis/pangolinRouterAbi";
import { USDCE_ADDRESS, WAVAX_ADDRESS } from "./constants";

describe("PangolinRouter", function () {
  let accounts: SignerWithAddress[];
  let pangolin: any; // TODO: fix
  const pangolinRouter = "0xE54Ca86531e17Ef3616d22Ca28b0D458b6C89106";

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
          [WAVAX_ADDRESS, USDCE_ADDRESS],
          wallet1,
          currentTime + 1000000000,
          { value: ethers.utils.parseEther("0.1") }
        );

      const usdceContract = await ethers.getContractAt("IERC20", USDCE_ADDRESS);
      const balanceOfUsdce = await usdceContract.balanceOf(wallet1);
      expect(balanceOfUsdce).not.eq(0)
    });
  });
});
