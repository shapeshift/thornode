
from coin import Coin

class Transaction:
    def __init__(self, _chain, _from, _to, _coins, _memo):
        self.chain = _chain
        self.toAddress = _to
        self.fromAddress = _from
        self.coins = _coins
        self.memo = _memo

    def __repr__(self):
        return "<Transaction %s ==> %s | %s | %s>" % (self.fromAddress, self.toAddress, self.coins, self.memo)

    def __str__(self):
        return "Transaction %s ==> %s | %s | %s" % (self.fromAddress, self.toAddress, self.coins, self.memo)
