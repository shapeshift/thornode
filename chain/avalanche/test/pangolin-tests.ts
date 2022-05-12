import { ethers } from "hardhat";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";
import { expect } from "chai";

describe("PangolinRouter", function () {
  let accounts: SignerWithAddress[];
  let pangolin;

  beforeEach(async () => {
    accounts = await ethers.getSigners();
    const Pangolin = await ethers.getContractFactory("PangolinRouter");
    pangolin = Pangolin.deploy("0xB31f66AA3C1e785363F0875A1B74E27b85FD66c7");
  });

  describe("initializePangolinRouter", function () {
    it("Should init", async () => {
      expect(pangolin.address).to.not.eq(ethers.constants.AddressZero);
    });
  });
});
