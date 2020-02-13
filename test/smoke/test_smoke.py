import unittest

import json
import pprint
from deepdiff import DeepDiff

from chains import Binance
from thorchain import ThorchainState
from breakpoint import Breakpoint

from common import Transaction, Coin
from smoke import txns

def get_balance(idx):
    """
    Retrieve expected balance with given id
    """
    with open('data/balances.json') as f:
        balances = json.load(f)
        for bal in balances:
            if idx == bal['TX']:
                return bal
    raise Exception("could not find idx")

class TestSmoke(unittest.TestCase):
    """
    This runs tests with a pre-determined list of transactions and an expected
    balance after each transaction (/data/balance.json). These transactions and
    balances were determined earlier via a google spreadsheet
    https://docs.google.com/spreadsheets/d/1sLK0FE-s6LInWijqKgxAzQk2RiSDZO1GL58kAD62ch0/edit#gid=439437407
    """
    def test_smoke(self):
        bnb = Binance() # init local binance chain
        thorchain = ThorchainState() # init local thorchain 

        for i, unit in enumerate(txns):
            txn, out = unit # get transaction and expected number of outbound transactions
            print("{} {}".format(i, txn))
            if txn.memo == "SEED":
                bnb.seed(txn.toAddress, txn.coins)
                continue
            else:
                bnb.transfer(txn) # send transfer on binance chain
                outbound = thorchain.handle(txn) # process transaction in thorchain
                for txn in outbound:
                    gas = bnb.transfer(txn) # send outbound txns back to Binance
                    thorchain.handle_gas(gas) # subtract gas from pool(s)

            # generated a snapshop picture of thorchain and bnb
            snap = Breakpoint(thorchain, bnb).snapshot(i, out)
            expected = get_balance(i) # get the expected balance from json file

            diff = DeepDiff(snap, expected, ignore_order=True) # empty dict if are equal
            if len(diff) > 0:
                print("Transaction:", i, txn)
                print(">>>>>> Expected")
                pprint.pprint(expected)
                print(">>>>>> Obtained")
                pprint.pprint(snap)
                print(">>>>>> DIFF")
                pprint.pprint(diff)
                raise Exception("did not match!")


if __name__ == '__main__':
    unittest.main()
