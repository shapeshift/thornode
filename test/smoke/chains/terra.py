import os
import time
import asyncio
import threading

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
        "USER-1": "vintage announce rapid clip spare stomach matter camp noble habit "
        + "beef amateur chimney time fuel machine culture end toe oval isolate "
        + "laptop solar gift",
        "PROVIDER-1": "discover blue crunch cart club fish airport crazy roast hybrid "
        + "scheme picnic veteran mango beach narrow luxury glory dynamic crawl symbol "
        + "win sell dress",
    }
    block_stats = {
        "tx_rate": 2000000,
        "tx_size": 1,
    }
    gas_price_factor = 1000000000
    gas_limit = 200000
    default_gas = 2000000

    def __init__(self, base_url):
        self.base_url = base_url
        self.lcd_client = LCDClient(base_url, "localterra")
        threading.Thread(target=self.scan_blocks, daemon=True).start()
        self.init_wallets()
        self.broadcast_fee_txs()

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
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        lcd_client = LCDClient(self.base_url, "localterra")
        height = int(lcd_client.tendermint.block_info()["block"]["header"]["height"])
        fee_cache = []
        while True:
            try:
                txs = lcd_client.tx.tx_infos_by_height(height)
                height += 1
                for tx in txs:
                    fee = tx.auth_info.fee
                    if len(fee.amount) != 1:
                        continue
                    fee_coin = fee.amount[0]
                    if fee_coin.denom != "uluna":
                        continue
                    gas = fee.gas_limit
                    amount = int(fee_coin.amount) * 100 * self.gas_price_factor
                    price = amount / gas
                    fee = price * self.gas_limit
                    fee = fee / self.gas_price_factor
                    fee_cache.append(fee)
                    if len(fee_cache) > 100:
                        fee_cache.pop(0)
                if len(fee_cache) != 100:
                    continue
                if (height - 1) % 10 == 0:
                    tx_rate = int(sum(fee_cache) / 100)
                    self.block_stats["tx_rate"] = tx_rate
            except Exception:
                continue
            finally:
                backoff = os.environ.get("BLOCK_SCANNER_BACKOFF", 0.3)
                time.sleep(backoff)

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

    def get_balance(self, account):
        """
        Get the balance account
        """
        coins = self.lcd_client.bank.balance(account)[0]
        result = []
        for coin in coins.to_list():
            symbol = coin.denom[1:].upper()
            if symbol == "USD":
                symbol = "UST"
            if symbol != "UST" and symbol != "LUNA":
                continue
            asset = f"{Terra.chain}.{symbol}"
            result.append(Coin(asset, coin.amount * 100))
        return result

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

        # create transaction
        tx = wallet.create_and_sign_tx(
            CreateTxOptions(
                msgs=[
                    MsgSend(
                        txn.from_address, txn.to_address, txn.coins[0].to_cosmos_terra()
                    )
                ],
                memo=txn.memo,
                fee=Fee(200000, "20000uluna"),  # gas 0.2uluna fee 0.02uluna,
            )
        )

        result = self.lcd_client.tx.broadcast(tx)
        txn.id = result.txhash
        txn.gas = [Coin("TERRA.LUNA", self.default_gas)]


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
