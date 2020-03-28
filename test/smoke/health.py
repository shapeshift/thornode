import argparse
import sys
import os
import logging

from segwit_addr import decode_address, address_from_public_key
from chains import MockBinance, Account
from thorchain import ThorchainClient
from midgard import MidgardClient
from common import Coin


# Init logging
logging.basicConfig(
    format="%(asctime)s | %(levelname)-8s | %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--thorchain", default="http://localhost:1317", help="Thorchain API url"
    )
    parser.add_argument(
        "--midgard", default="http://localhost:8080", help="Midgard API url"
    )
    parser.add_argument(
        "--binance", default="http://localhost:26660", help="Mock binance server"
    )

    args = parser.parse_args()

    health = Health(args.thorchain, args.midgard, args.binance)
    try:
        health.run()
    except Exception as e:
        logging.error(e)
        sys.exit(1)


class Health:
    def __init__(self, thor, midgard, binance):
        self.errors = []
        self.thorchain_client = ThorchainClient(thor)
        self.thorchain_pools = []
        self.thorchain_asgard_vaults = []

        self.midgard_client = MidgardClient(midgard)
        self.midgard_pools = []

        self.binance_client = MockBinance(binance)
        self.binance_accounts = []

    def run(self):
        """Run health checks

        - check pools state between midgard and thorchain

        """
        self.retrieve_data()
        self.check_pools()
        self.check_asgard_vault()
        self.check_errors()

    def check_errors(self):
        """Check errors and exit accordingly.
        """
        for error in self.errors:
            logging.error(error)

        if len(self.errors):
            self.errors = []
            raise Exception("Health checks failed")

    def retrieve_data(self):
        """Retrieve data from APIs needed to run health checks.
        """
        self.thorchain_asgard_vaults = self.thorchain_client.get_asgard_vaults()
        for vault in self.thorchain_asgard_vaults:
            if vault["coins"]:
                vault["coins"] = [Coin.from_dict(c) for c in vault["coins"]]

        self.binance_accounts = []
        accounts = self.binance_client.accounts()
        for acct in accounts:
            account = Account(acct["address"])
            if acct["balances"]:
                account.balances = [
                    Coin(b["denom"], b["amount"]) for b in acct["balances"]
                ]
                self.binance_accounts.append(account)

        self.thorchain_pools = self.thorchain_client.get_pools()
        if len(self.thorchain_pools) == 0:
            return
        pool_assets = [p["asset"] for p in self.thorchain_pools]
        self.midgard_pools = self.midgard_client.get_pool(pool_assets)

    def get_midgard_pool(self, asset):
        """Get midgard pool from class member.

        :param str asset: Asset name
        :returns: pool

        """
        for pool in self.midgard_pools:
            if pool["asset"] == asset:
                return pool

    def check_pools(self):
        """Check pools state between Midgard and Thorchain APIs.
        """
        for tpool in self.thorchain_pools:
            asset = tpool["asset"]
            mpool = self.get_midgard_pool(asset)

            # Thorchain Coins
            trune_coin = Coin("RUNE-A1F", tpool["balance_rune"])
            tasset_coin = Coin(asset, tpool["balance_asset"])

            # Midgard Coins
            mrune_coin = Coin("RUNE-A1F", mpool["runeDepth"])
            masset_coin = Coin(asset, mpool["assetDepth"])

            # Check balances
            if trune_coin != mrune_coin:
                self.errors.append(
                    Exception(
                        f"Bad Midgard pool balance: BNB.RUNE-A1F "
                        f"{mrune_coin} != {trune_coin}"
                    )
                )

            if tasset_coin != masset_coin:
                self.errors.append(
                    Exception(
                        f"Bad Midgard pool balance: {asset} "
                        f"{masset_coin} != {tasset_coin}"
                    )
                )

            # Check pool units
            mpool_units = int(mpool["poolUnits"])
            tpool_units = int(tpool["pool_units"])
            if mpool_units != tpool_units:
                self.errors.append(
                    Exception(
                        f"Bad Midgard pool units: " f"{mpool_units} != {tpool_units}"
                    )
                )

    def get_vault_address_from_pubkey(self, pubkey, hrp="tbnb"):
        """
        Get vault address for a specific hrp (human readable part)
        bech32 encoded from a Bech32 encoded public key(secp256k1).
        The vault pubkey is Bech32 encoded and amino encoded which means
        when we bech32 decode, we also need to get rid of the first 5 bytes
        used by amino encoding to figure out a type to unmarshal correctly.
        Only then we get the raw pubkey bytes format secp256k1.

        :param string pubkey: public key Bech32 encoded & amino encoded
        :param string hrp: human readable part of the bech32 encoded result
        :returns: string bech32 encoded address

        """
        raw_pubkey = decode_address(pubkey)[5:]
        return address_from_public_key(raw_pubkey, hrp)

    def check_binance_accounts(self, vault):
        vault_addr = self.get_vault_address_from_pubkey(vault["pub_key"])
        for acct in self.binance_accounts:
            if acct.address != vault_addr:
                continue
            for bcoin in acct.balances:
                for vcoin in vault["coins"]:
                    if vcoin.asset != bcoin.asset:
                        continue
                    if vcoin != bcoin:
                        self.errors.append(
                            Exception(
                                f"Bad Asgard vault balance: {vcoin.asset} "
                                f"{vcoin} != {bcoin} (Binance balance)"
                            )
                        )

    def check_asgard_vault(self):
        for vault in self.thorchain_asgard_vaults:
            # Check Binance balances
            self.check_binance_accounts(vault)


if __name__ == "__main__":
    main()
