// SPDX-License-Identifier: MIT
pragma solidity 0.8.9;

import "hardhat/console.sol";

// ARC-20 Interface
interface iARC20 {
    function balanceOf(address) external view returns (uint256);

    function burn(uint256) external;
}

// ROUTER Interface
interface iROUTER {
    function depositWithExpiry(
        address,
        address,
        uint256,
        string calldata,
        uint256
    ) external;
}

// THORChain_Router is managed by THORChain Vaults
contract AvaxRouter {
    struct Coin {
        address asset;
        uint256 amount;
    }

    // Vault allowance for each asset
    mapping(address => mapping(address => uint256)) private _vaultAllowance;

    uint256 private constant _NOT_ENTERED = 1;
    uint256 private constant _ENTERED = 2;
    uint256 private _status;

    // Emitted for all deposits, the memo distinguishes for swap, add, remove, donate etc
    event Deposit(
        address indexed to,
        address indexed asset,
        uint256 amount,
        string memo
    );

    // Emitted for all outgoing transfers, the vault dictates who sent it, memo used to track.
    event TransferOut(
        address indexed vault,
        address indexed to,
        address asset,
        uint256 amount,
        string memo
    );

    // Emitted for all outgoing transferAndCalls, the vault dictates who sent it, memo used to track.
    event TransferOutAndCall(
        address indexed vault,
        address target,
        uint256 amount,
        address finalAsset,
        address to,
        uint256 amountOutMin,
        string memo
    );

    // Changes the spend allowance between vaults
    event TransferAllowance(
        address indexed oldVault,
        address indexed newVault,
        address asset,
        uint256 amount,
        string memo
    );

    // Specifically used to batch send the entire vault assets
    event VaultTransfer(
        address indexed oldVault,
        address indexed newVault,
        Coin[] coins,
        string memo
    );

    modifier nonReentrant() {
        require(_status != _ENTERED, "ReentrancyGuard: reentrant call");
        _status = _ENTERED;
        _;
        _status = _NOT_ENTERED;
    }

    constructor() {
        _status = _NOT_ENTERED;
    }

    // Deposit with Expiry (preferred)
    function depositWithExpiry(
        address payable vault,
        address asset,
        uint256 amount,
        string memory memo,
        uint256 expiration
    ) external payable {
        require(block.timestamp < expiration, "THORChain_Router: expired");
        deposit(vault, asset, amount, memo);
    }

    // Deposit an asset with a memo. Avax is forwarded, ARC-20 stays in ROUTER
    function deposit(
        address payable vault,
        address asset,
        uint256 amount,
        string memory memo
    ) public payable nonReentrant {
        uint256 safeAmount;
        if (asset == address(0)) {
            safeAmount = msg.value;
            bool success = vault.send(safeAmount);
            require(success);
        } else {
            require(msg.value == 0, "unexpected avax"); // protect user from accidentally locking up AVAX

            safeAmount = safeTransferFrom(asset, amount); // Transfer asset
            _vaultAllowance[vault][asset] += safeAmount; // Credit to chosen vault
        }
        emit Deposit(vault, asset, safeAmount, memo);
    }

    //############################## ALLOWANCE TRANSFERS ##############################

    // Use for "moving" assets between vaults (asgard<>ygg), as well "churning" to a new Asgard
    function transferAllowance(
        address router,
        address newVault,
        address asset,
        uint256 amount,
        string memory memo
    ) external nonReentrant {
        if (router == address(this)) {
            _adjustAllowances(newVault, asset, amount);
            emit TransferAllowance(msg.sender, newVault, asset, amount, memo);
        } else {
            _routerDeposit(router, newVault, asset, amount, memo);
        }
    }

    //############################## ASSET TRANSFERS ##############################

    // Any vault calls to transfer any asset to any recipient.
    // Note: Contract recipients of AVAX are only given 2300 Gas to complete execution.
    function transferOut(
        address payable to,
        address asset,
        uint256 amount,
        string memory memo
    ) public payable nonReentrant {
        uint256 safeAmount;
        if (asset == address(0)) {
            safeAmount = msg.value;
            bool success = to.send(safeAmount); // Send AVAX.
            if (!success) {
                payable(address(msg.sender)).transfer(safeAmount); // For failure, bounce back to Yggdrasil & continue.
            }
        } else {
            _vaultAllowance[msg.sender][asset] -= amount; // Reduce allowance
            (bool success, bytes memory data) = asset.call(
                abi.encodeWithSignature("transfer(address,uint256)", to, amount)
            );
            require(success && (data.length == 0 || abi.decode(data, (bool))));
            safeAmount = amount;
        }
        emit TransferOut(msg.sender, to, asset, safeAmount, memo);
    }

    // Any vault calls to transferAndCall on a target contract that conforms with "swapOut(address,address,uint256)"
    // Example Memo: "~1b3:AVAX.0xFinalToken:0xTo:"
    // Target is fuzzy-matched to the last three digits of whitelisted aggregators
    // FinalToken, To, amountOutMin come from originating memo
    // Memo passed in here is the "OUT:HASH" type
    function transferOutAndCall(
        address payable target,
        address finalToken,
        address to,
        uint256 amountOutMin,
        string memory memo
    ) public payable nonReentrant {
        uint256 _safeAmount = msg.value;
        (bool arc20Success, ) = target.call{value: _safeAmount}(
            abi.encodeWithSignature(
                "swapOut(address,address,uint256)",
                finalToken,
                to,
                amountOutMin
            )
        );
        if (!arc20Success) {
            bool avaxSuccess = payable(to).send(_safeAmount); // If can't swap, just send the recipient the AVAX
            if (!avaxSuccess) {
                payable(address(msg.sender)).transfer(_safeAmount); // For failure, bounce back to Yggdrasil & continue.
            }
        }
        emit TransferOutAndCall(
            msg.sender,
            target,
            _safeAmount,
            finalToken,
            to,
            amountOutMin,
            memo
        );
    }

    //############################## VAULT MANAGEMENT ##############################

    // A vault can call to "return" all assets to an asgard, including AVAX.
    function returnVaultAssets(
        address router,
        address payable asgard,
        Coin[] memory coins,
        string memory memo
    ) external payable nonReentrant {
        if (router == address(this)) {
            for (uint256 i = 0; i < coins.length; i++) {
                _adjustAllowances(asgard, coins[i].asset, coins[i].amount);
            }
            emit VaultTransfer(msg.sender, asgard, coins, memo); // Does not include AVAX.
        } else {
            for (uint256 i = 0; i < coins.length; i++) {
                _routerDeposit(
                    router,
                    asgard,
                    coins[i].asset,
                    coins[i].amount,
                    memo
                );
            }
        }
        bool success = asgard.send(msg.value);
        require(success);
    }

    //############################## HELPERS ##############################

    function vaultAllowance(address vault, address token)
        public
        view
        returns (uint256 amount)
    {
        return _vaultAllowance[vault][token];
    }

    // Safe transferFrom in case asset charges transfer fees
    function safeTransferFrom(address _asset, uint256 _amount)
        internal
        returns (uint256 amount)
    {
        uint256 _startBal = iARC20(_asset).balanceOf(address(this));
        (bool success, bytes memory data) = _asset.call(
            abi.encodeWithSignature(
                "transferFrom(address,address,uint256)",
                msg.sender,
                address(this),
                _amount
            )
        );
        require(
            success && (data.length == 0 || abi.decode(data, (bool))),
            "Failed To TransferFrom"
        );
        return (iARC20(_asset).balanceOf(address(this)) - _startBal);
    }

    // Decrements and Increments Allowances between two vaults
    function _adjustAllowances(
        address _newVault,
        address _asset,
        uint256 _amount
    ) internal {
        _vaultAllowance[msg.sender][_asset] -= _amount;
        _vaultAllowance[_newVault][_asset] += _amount;
    }

    // Adjust allowance and forwards funds to new router, credits allowance to desired vault
    function _routerDeposit(
        address _router,
        address _vault,
        address _asset,
        uint256 _amount,
        string memory _memo
    ) internal {
        _vaultAllowance[msg.sender][_asset] -= _amount;
        (bool success, ) = _asset.call(
            abi.encodeWithSignature(
                "approve(address,uint256)",
                _router,
                _amount
            )
        ); // Approve to transfer
        require(success);
        iROUTER(_router).depositWithExpiry(
            _vault,
            _asset,
            _amount,
            _memo,
            type(uint256).max
        ); // Transfer by depositing
    }
}
