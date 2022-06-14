// SPDX-License-Identifier: AGPL-3.0-or-later
pragma solidity 0.8.9;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

// ROUTER Interface
interface IRouter {
    function depositWithExpiry(
        address payable vault,
        address asset,
        uint256 amount,
        string memory memo,
        uint256 expiration
    ) external payable;
}

// Sushi Interface
interface ISwapRouter {
    function swapExactTokensForAVAX(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external;

    function swapExactAVAXForTokens(
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external payable;
}

// THORChain_Aggregator is permissionless
contract AvaxAggregator {
    using SafeERC20 for IERC20;

    uint256 private constant _NOT_ENTERED = 1;
    uint256 private constant _ENTERED = 2;
    uint256 private _status;

    address private AVAX = address(0);
    address public WAVAX;
    ISwapRouter public swapRouter;

    modifier nonReentrant() {
        require(_status != _ENTERED, "ReentrancyGuard: reentrant call");
        _status = _ENTERED;
        _;
        _status = _NOT_ENTERED;
    }

    constructor(address _wavax, address _swapRouter) {
        _status = _NOT_ENTERED;
        WAVAX = _wavax;
        swapRouter = ISwapRouter(_swapRouter);
    }

    receive() external payable {}

    //############################## IN ##############################

    function swapIn(
        address tcVault,
        address tcRouter,
        string calldata tcMemo,
        address token,
        uint256 amount,
        uint256 amountOutMin,
        uint256 deadline
    ) public nonReentrant {
        uint256 startBal = IERC20(token).balanceOf(address(this));

        IERC20(token).safeTransferFrom(msg.sender, address(this), amount); // Transfer asset
        IERC20(token).safeApprove(address(swapRouter), amount);

        uint256 safeAmount = (IERC20(token).balanceOf(address(this)) -
            startBal);

        address[] memory path = new address[](2);
        path[0] = token;
        path[1] = WAVAX;

        swapRouter.swapExactTokensForAVAX(
            safeAmount,
            amountOutMin,
            path,
            address(this),
            deadline
        );
        safeAmount = address(this).balance;
        IRouter(tcRouter).depositWithExpiry{value: safeAmount}(
            payable(tcVault),
            AVAX,
            safeAmount,
            tcMemo,
            deadline
        );
    }

    //############################## OUT ##############################

    function swapOut(
        address token,
        address to,
        uint256 amountOutMin
    ) public payable nonReentrant {
        address[] memory path = new address[](2);
        path[0] = WAVAX;
        path[1] = token;
        swapRouter.swapExactAVAXForTokens{value: msg.value}(
            amountOutMin,
            path,
            to,
            type(uint256).max
        );
    }
}
