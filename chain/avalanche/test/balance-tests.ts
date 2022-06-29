import { ethers, getNamedAccounts, network, deployments } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";
import { Contract, Signer } from "ethers";
import { pangolinRouterAbi } from "./abis/pangolinRouterAbi";
import { USDCE_ADDRESS, WAVAX_ADDRESS } from "./constants";
import { AvaxAggregator, AvaxRouter } from "../typechain-types";
import ERC20 from "@openzeppelin/contracts/build/contracts/ERC20.json";

describe("AvaxAggregator", function () {
  let accounts: SignerWithAddress[];
  let avaxAggregator: AvaxAggregator;
  let avaxRouter: AvaxRouter;
  let usdceToken: Contract;
  let pangolin: any;
  const pangolinRouter = "0xE54Ca86531e17Ef3616d22Ca28b0D458b6C89106";

  beforeEach(async () => {
    accounts = await ethers.getSigners();
    await deployments.fixture();

    pangolin = new ethers.Contract(
      pangolinRouter,
      pangolinRouterAbi,
      accounts[0]
    );
    const avaxRouterDeployment = await ethers.getContractFactory("AvaxRouter");
    avaxRouter = await avaxRouterDeployment.deploy();
    const avaxAggregatorDeployment = await ethers.getContractFactory(
      "AvaxAggregator"
    );
    avaxAggregator = await avaxAggregatorDeployment.deploy(
      WAVAX_ADDRESS,
      pangolinRouter
    );
    usdceToken = new ethers.Contract(USDCE_ADDRESS, ERC20.abi, accounts[0]);
  });

  describe("Check Balances", function () {
    it.only("Balance of USDC.e", async () => {
      const { admin } = await getNamedAccounts();

      const usdceContract = await ethers.getContractAt("IERC20", USDCE_ADDRESS);
      const balanceOfUsdce = await usdceContract.balanceOf(admin);
      console.log(balanceOfUsdce)
    });
  });
});
