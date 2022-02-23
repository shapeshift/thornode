import time

from terra_sdk.client.lcd import LCDClient
from terra_sdk.key.mnemonic import MnemonicKey
from terra_sdk.core.fee import Fee
from terra_sdk.core.bank import MsgSend
from terra_sdk.client.lcd.api.tx import CreateTxOptions

from utils.segwit_addr import address_from_public_key
from utils.common import Coin, HttpClient, get_rune_asset, Asset
from chains.aliases import aliases_terra, get_aliases, get_alias_address
from chains.chain import GenericChain

RUNE = get_rune_asset()


class MockTerra(HttpClient):
    """
    An client implementation for a regtest bitcoin server
    """

    mnemonic = {
        "CONTRIB": "satisfy adjust timber high purchase tuition stool "
        + "faith fine install that you unaware feed domain "
        + "license impose boss human eager hat rent enjoy dawn",
        "MASTER": "notice oak worry limit wrap speak medal online "
        + "prefer cluster roof addict wrist behave treat actual "
        + "wasp year salad speed social layer crew genius",
        "USER-1": "quality vacuum heart guard buzz spike sight "
        + "swarm shove special gym robust assume sudden deposit "
        + "grid alcohol choice devote leader tilt noodle tide penalty",
        "PROVIDER-1": "symbol force gallery make bulk round subway "
        + "violin worry mixture penalty kingdom boring survey tool "
        + "fringe patrol sausage hard admit remember broken alien absorb",
    }
    block_stats = {
        "tx_rate": 2000000,
        "tx_size": 1,
    }
    gas_price_factor = 1e9
    gas_limit = 200000
    default_gas = 2000000

    def __init__(self, base_url):
        self.lcd_client = LCDClient(base_url, "localterra")
        self.init_wallets()
        self.broadcast_fee_txs()
        # threading.Thread(target=self.scan_blocks, daemon=True).start()

    def init_wallets(self):
        """
        Init wallet instances
        """
        self.wallets = {}
        for alias in self.mnemonic:
            mk = MnemonicKey(mnemonic=self.mnemonic[alias])
            self.wallets[alias] = self.lcd_client.wallet(mk)

    def broadcast_fee_txs(self):
        """
        Generate 100 txs to build cache for bifrost to estimate fees
        """
        sequence = self.wallets["CONTRIB"].sequence() - 1
        for x in range(100):
            sequence += 1
            tx = self.wallets["CONTRIB"].create_and_sign_tx(
                CreateTxOptions(
                    msgs=[
                        MsgSend(
                            get_alias_address(Terra.chain, "CONTRIB"),
                            get_alias_address(Terra.chain, "MASTER"),
                            "10000uluna",  # send 0.01 luna
                        )
                    ],
                    sequence=sequence,
                    memo="fee generation",
                    fee=Fee(200000, "20000uluna"),  # fee 0.02 luna
                )
            )
            self.lcd_client.tx.broadcast_sync(tx)

    def scan_blocks(self):
        height = int(self.get_block()["block"]["header"]["height"])
        fee_cache = []
        while True:
            try:
                txs = self.get_block_txs(height)
                for tx in txs:
                    fee = tx.auth_info.fee
                    if len(fee.amount) != 1:
                        continue
                    fee = fee.amount[0]
                    if fee.denom != "uluna":
                        continue
                    gas = fee.gas_limit
                    amount = fee * self.gas_price_factor
                    price = amount / gas
                    fee = price * self.gas_limit
                    fee = fee / self.gas_price_factor
                    fee_cache.append(fee)
                    if len(fee_cache) > 100:
                        fee_cache.pop(0)
                if len(fee_cache) != 100:
                    continue
                self.block_stats["tx_rate"] = sum(fee_cache) / 100
            except Exception:
                continue
            finally:
                height += 1
                time.sleep(1)

    @classmethod
    def get_address_from_pubkey(cls, pubkey, prefix="terra"):
        """
        Get bnb testnet address for a public key

        :param string pubkey: public key
        :returns: string bech32 encoded address
        """
        return address_from_public_key(pubkey, prefix)

    def get_block(self, block_height=None):
        """
        Get the block data for a height
        """
        return self.lcd_client.tendermint.block_info(block_height)

    def get_block_txs(self, block_height=None):
        """
        Get the block txs data for a height
        """
        return self.lcd_client.tx.tx_infos_by_height(block_height)

    def set_vault_address_by_pubkey(self, pubkey):
        """
        Set vault adddress by pubkey
        """
        self.set_vault_address(self.get_address_from_pubkey(pubkey))

    def set_vault_address(self, addr):
        """
        Set the vault bnb address
        """
        aliases_terra["VAULT"] = addr

    def transfer(self, txn):
        """
        Make a transaction/transfer on local Terra
        """
        if not isinstance(txn.coins, list):
            txn.coins = [txn.coins]

        wallet = self.wallets[txn.from_address]

        if txn.to_address in get_aliases():
            txn.to_address = get_alias_address(txn.chain, txn.to_address)

        if txn.from_address in get_aliases():
            txn.from_address = get_alias_address(txn.chain, txn.from_address)

        # update memo with actual address (over alias name)
        is_synth = txn.is_synth()
        for alias in get_aliases():
            chain = txn.chain
            asset = txn.get_asset_from_memo()
            if asset:
                chain = asset.get_chain()
            # we use RUNE BNB address to identify a cross chain liqudity provision
            if txn.memo.startswith("ADD") or is_synth:
                chain = RUNE.get_chain()
            addr = get_alias_address(chain, alias)
            txn.memo = txn.memo.replace(alias, addr)

        fee = Fee(200000, "20000uluna")  # gas 0.2uluna fee 0.02uluna
        if txn.coins[0].asset.is_ust():
            fee = Fee(200000, "100000uluna,100000uusd")

        # create transaction
        tx = wallet.create_and_sign_tx(
            CreateTxOptions(
                msgs=[
                    MsgSend(txn.from_address, txn.to_address, txn.coins[0].to_cosmos())
                ],
                memo=txn.memo,
                fee=fee,
            )
        )

        result = self.lcd_client.tx.broadcast(tx)
        txn.id = result.txhash
        # txn.gas = [Coin("TERRA.LUNA", self.default_gas)]


class Terra(GenericChain):
    """
    A local simple implementation of Terra chain
    """

    name = "Terra"
    chain = "TERRA"
    coin = Asset("TERRA.LUNA")
    rune_fee = 2000000

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        Calculate gas according to RUNE thorchain fee
        """
        return Coin(cls.coin, 2000000)
