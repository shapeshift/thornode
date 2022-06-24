import { ethers } from "hardhat";

async function main() {
  const wavaxAddress = "0xb31f66aa3c1e785363f0875a1b74e27b85fd66c7";
  const pangolinRouter = "0xE54Ca86531e17Ef3616d22Ca28b0D458b6C89106";

  const AvaxRouter = await ethers.getContractFactory("AvaxRouter");
  const avaxRouter = await AvaxRouter.deploy();
  await avaxRouter.deployed();

  console.log("AvaxRouter deployed to:", avaxRouter.address);

  const AvaxAggregator = await ethers.getContractFactory("AvaxAggregator");
  const avaxAggregator = await AvaxAggregator.deploy(
    wavaxAddress,
    pangolinRouter
  );
  await avaxAggregator.deployed();

  console.log("AvaxAggregator deployed to:", avaxAggregator.address);
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
