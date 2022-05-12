// SPDX-License-Identifier: AGPL-3.0-or-later
pragma solidity 0.8.9;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "../interfaces/IAVAX.sol";

contract PangolinRouter {
    using SafeERC20 for IERC20;
    address private WAVAX;
    uint256 one = 1 * 10**18; // TODO: right decimals?

    constructor(address _wavax) {
        WAVAX = _wavax;
    }

    receive() external payable {}


    function swapExactTokensForAVAX(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external virtual {
        require(path[path.length - 1] == WAVAX, "PangolinRouter: INVALID_PATH");
        uint256[1] memory amounts = [one];
        require(
            amounts[amounts.length - 1] >= amountOutMin,
            "PangolinRouter: INSUFFICIENT_OUTPUT_AMOUNT"
        );
        IERC20(path[0]).safeTransferFrom(msg.sender, address(this), amountIn);
        IAVAX(WAVAX).withdraw(amounts[amounts.length - 1]);
        (bool success, ) = to.call{value: amounts[amounts.length - 1]}(
            new bytes(0)
        );
        require(success, "AVAX Transfer Failed");
    }

    function swapExactAVAXForTokens(
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external payable virtual {
        require(path[0] == WAVAX, "PangolinRouter: INVALID_PATH");
        uint256[1] memory amounts = [one];
        require(
            amounts[amounts.length - 1] >= amountOutMin,
            "PangolinRouter: INSUFFICIENT_OUTPUT_AMOUNT"
        );
        IAVAX(WAVAX).deposit{value: one}();
        IERC20(path[1]).safeTransfer(to, one);
    }
}
