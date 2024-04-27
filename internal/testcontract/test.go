// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package testcontract

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// TestcontractMetaData contains all meta data concerning the Testcontract contract.
var TestcontractMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"name\":\"ErrorNoArgs\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"number\",\"type\":\"uint256\"}],\"name\":\"ErrorOneArg\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"EventNoArgs\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"number\",\"type\":\"uint256\"}],\"name\":\"EventOneArg\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"revertNoMessage\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"revertWithMessage\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"revertWithNoArgsError\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"revertWithOneArgError\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"succeedNoEvents\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"succeedOneEvent\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b506102bc806100206000396000f3fe608060405234801561001057600080fd5b50600436106100625760003560e01c80630d60c8ae14610067578063185c38a4146100715780634740a9ce1461007b5780637050f4c0146100855780639fe554331461008f578063f583dabb14610099575b600080fd5b61006f6100a3565b005b6100796100d5565b005b610083610118565b005b61008d610156565b005b610097610158565b005b6100a1610165565b005b6040517fc1b1c82c00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b6000610116576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161010d906101fc565b60405180910390fd5b565b602a6040517fac91dbf900000000000000000000000000000000000000000000000000000000815260040161014d919061026b565b60405180910390fd5b565b600061016357600080fd5b565b7fe59bed36a0b9e33df7f92823525e084566de6674a92ed3c1560b217c5e37967d602a604051610195919061026b565b60405180910390a1565b600082825260208201905092915050565b7f4d792072657175697265206d6573736167650000000000000000000000000000600082015250565b60006101e660128361019f565b91506101f1826101b0565b602082019050919050565b60006020820190508181036000830152610215816101d9565b9050919050565b6000819050919050565b6000819050919050565b6000819050919050565b600061025561025061024b8461021c565b610230565b610226565b9050919050565b6102658161023a565b82525050565b6000602082019050610280600083018461025c565b9291505056fea264697066735822122036a0d816724ec37acbc20d4239ac67a94bb0d3e5342382713b073382e88d589c64736f6c63430008110033",
}

// TestcontractABI is the input ABI used to generate the binding from.
// Deprecated: Use TestcontractMetaData.ABI instead.
var TestcontractABI = TestcontractMetaData.ABI

// TestcontractBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use TestcontractMetaData.Bin instead.
var TestcontractBin = TestcontractMetaData.Bin

// DeployTestcontract deploys a new Ethereum contract, binding an instance of Testcontract to it.
func DeployTestcontract(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Testcontract, error) {
	parsed, err := TestcontractMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(TestcontractBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Testcontract{TestcontractCaller: TestcontractCaller{contract: contract}, TestcontractTransactor: TestcontractTransactor{contract: contract}, TestcontractFilterer: TestcontractFilterer{contract: contract}}, nil
}

// Testcontract is an auto generated Go binding around an Ethereum contract.
type Testcontract struct {
	TestcontractCaller     // Read-only binding to the contract
	TestcontractTransactor // Write-only binding to the contract
	TestcontractFilterer   // Log filterer for contract events
}

// TestcontractCaller is an auto generated read-only Go binding around an Ethereum contract.
type TestcontractCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TestcontractTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TestcontractTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TestcontractFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type TestcontractFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TestcontractSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TestcontractSession struct {
	Contract     *Testcontract     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TestcontractCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TestcontractCallerSession struct {
	Contract *TestcontractCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// TestcontractTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TestcontractTransactorSession struct {
	Contract     *TestcontractTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// TestcontractRaw is an auto generated low-level Go binding around an Ethereum contract.
type TestcontractRaw struct {
	Contract *Testcontract // Generic contract binding to access the raw methods on
}

// TestcontractCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TestcontractCallerRaw struct {
	Contract *TestcontractCaller // Generic read-only contract binding to access the raw methods on
}

// TestcontractTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TestcontractTransactorRaw struct {
	Contract *TestcontractTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTestcontract creates a new instance of Testcontract, bound to a specific deployed contract.
func NewTestcontract(address common.Address, backend bind.ContractBackend) (*Testcontract, error) {
	contract, err := bindTestcontract(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Testcontract{TestcontractCaller: TestcontractCaller{contract: contract}, TestcontractTransactor: TestcontractTransactor{contract: contract}, TestcontractFilterer: TestcontractFilterer{contract: contract}}, nil
}

// NewTestcontractCaller creates a new read-only instance of Testcontract, bound to a specific deployed contract.
func NewTestcontractCaller(address common.Address, caller bind.ContractCaller) (*TestcontractCaller, error) {
	contract, err := bindTestcontract(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &TestcontractCaller{contract: contract}, nil
}

// NewTestcontractTransactor creates a new write-only instance of Testcontract, bound to a specific deployed contract.
func NewTestcontractTransactor(address common.Address, transactor bind.ContractTransactor) (*TestcontractTransactor, error) {
	contract, err := bindTestcontract(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &TestcontractTransactor{contract: contract}, nil
}

// NewTestcontractFilterer creates a new log filterer instance of Testcontract, bound to a specific deployed contract.
func NewTestcontractFilterer(address common.Address, filterer bind.ContractFilterer) (*TestcontractFilterer, error) {
	contract, err := bindTestcontract(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &TestcontractFilterer{contract: contract}, nil
}

// bindTestcontract binds a generic wrapper to an already deployed contract.
func bindTestcontract(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := TestcontractMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Testcontract *TestcontractRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Testcontract.Contract.TestcontractCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Testcontract *TestcontractRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Testcontract.Contract.TestcontractTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Testcontract *TestcontractRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Testcontract.Contract.TestcontractTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Testcontract *TestcontractCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Testcontract.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Testcontract *TestcontractTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Testcontract.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Testcontract *TestcontractTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Testcontract.Contract.contract.Transact(opts, method, params...)
}

// RevertNoMessage is a paid mutator transaction binding the contract method 0x9fe55433.
//
// Solidity: function revertNoMessage() returns()
func (_Testcontract *TestcontractTransactor) RevertNoMessage(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Testcontract.contract.Transact(opts, "revertNoMessage")
}

// RevertNoMessage is a paid mutator transaction binding the contract method 0x9fe55433.
//
// Solidity: function revertNoMessage() returns()
func (_Testcontract *TestcontractSession) RevertNoMessage() (*types.Transaction, error) {
	return _Testcontract.Contract.RevertNoMessage(&_Testcontract.TransactOpts)
}

// RevertNoMessage is a paid mutator transaction binding the contract method 0x9fe55433.
//
// Solidity: function revertNoMessage() returns()
func (_Testcontract *TestcontractTransactorSession) RevertNoMessage() (*types.Transaction, error) {
	return _Testcontract.Contract.RevertNoMessage(&_Testcontract.TransactOpts)
}

// RevertWithMessage is a paid mutator transaction binding the contract method 0x185c38a4.
//
// Solidity: function revertWithMessage() returns()
func (_Testcontract *TestcontractTransactor) RevertWithMessage(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Testcontract.contract.Transact(opts, "revertWithMessage")
}

// RevertWithMessage is a paid mutator transaction binding the contract method 0x185c38a4.
//
// Solidity: function revertWithMessage() returns()
func (_Testcontract *TestcontractSession) RevertWithMessage() (*types.Transaction, error) {
	return _Testcontract.Contract.RevertWithMessage(&_Testcontract.TransactOpts)
}

// RevertWithMessage is a paid mutator transaction binding the contract method 0x185c38a4.
//
// Solidity: function revertWithMessage() returns()
func (_Testcontract *TestcontractTransactorSession) RevertWithMessage() (*types.Transaction, error) {
	return _Testcontract.Contract.RevertWithMessage(&_Testcontract.TransactOpts)
}

// RevertWithNoArgsError is a paid mutator transaction binding the contract method 0x0d60c8ae.
//
// Solidity: function revertWithNoArgsError() returns()
func (_Testcontract *TestcontractTransactor) RevertWithNoArgsError(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Testcontract.contract.Transact(opts, "revertWithNoArgsError")
}

// RevertWithNoArgsError is a paid mutator transaction binding the contract method 0x0d60c8ae.
//
// Solidity: function revertWithNoArgsError() returns()
func (_Testcontract *TestcontractSession) RevertWithNoArgsError() (*types.Transaction, error) {
	return _Testcontract.Contract.RevertWithNoArgsError(&_Testcontract.TransactOpts)
}

// RevertWithNoArgsError is a paid mutator transaction binding the contract method 0x0d60c8ae.
//
// Solidity: function revertWithNoArgsError() returns()
func (_Testcontract *TestcontractTransactorSession) RevertWithNoArgsError() (*types.Transaction, error) {
	return _Testcontract.Contract.RevertWithNoArgsError(&_Testcontract.TransactOpts)
}

// RevertWithOneArgError is a paid mutator transaction binding the contract method 0x4740a9ce.
//
// Solidity: function revertWithOneArgError() returns()
func (_Testcontract *TestcontractTransactor) RevertWithOneArgError(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Testcontract.contract.Transact(opts, "revertWithOneArgError")
}

// RevertWithOneArgError is a paid mutator transaction binding the contract method 0x4740a9ce.
//
// Solidity: function revertWithOneArgError() returns()
func (_Testcontract *TestcontractSession) RevertWithOneArgError() (*types.Transaction, error) {
	return _Testcontract.Contract.RevertWithOneArgError(&_Testcontract.TransactOpts)
}

// RevertWithOneArgError is a paid mutator transaction binding the contract method 0x4740a9ce.
//
// Solidity: function revertWithOneArgError() returns()
func (_Testcontract *TestcontractTransactorSession) RevertWithOneArgError() (*types.Transaction, error) {
	return _Testcontract.Contract.RevertWithOneArgError(&_Testcontract.TransactOpts)
}

// SucceedNoEvents is a paid mutator transaction binding the contract method 0x7050f4c0.
//
// Solidity: function succeedNoEvents() returns()
func (_Testcontract *TestcontractTransactor) SucceedNoEvents(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Testcontract.contract.Transact(opts, "succeedNoEvents")
}

// SucceedNoEvents is a paid mutator transaction binding the contract method 0x7050f4c0.
//
// Solidity: function succeedNoEvents() returns()
func (_Testcontract *TestcontractSession) SucceedNoEvents() (*types.Transaction, error) {
	return _Testcontract.Contract.SucceedNoEvents(&_Testcontract.TransactOpts)
}

// SucceedNoEvents is a paid mutator transaction binding the contract method 0x7050f4c0.
//
// Solidity: function succeedNoEvents() returns()
func (_Testcontract *TestcontractTransactorSession) SucceedNoEvents() (*types.Transaction, error) {
	return _Testcontract.Contract.SucceedNoEvents(&_Testcontract.TransactOpts)
}

// SucceedOneEvent is a paid mutator transaction binding the contract method 0xf583dabb.
//
// Solidity: function succeedOneEvent() returns()
func (_Testcontract *TestcontractTransactor) SucceedOneEvent(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Testcontract.contract.Transact(opts, "succeedOneEvent")
}

// SucceedOneEvent is a paid mutator transaction binding the contract method 0xf583dabb.
//
// Solidity: function succeedOneEvent() returns()
func (_Testcontract *TestcontractSession) SucceedOneEvent() (*types.Transaction, error) {
	return _Testcontract.Contract.SucceedOneEvent(&_Testcontract.TransactOpts)
}

// SucceedOneEvent is a paid mutator transaction binding the contract method 0xf583dabb.
//
// Solidity: function succeedOneEvent() returns()
func (_Testcontract *TestcontractTransactorSession) SucceedOneEvent() (*types.Transaction, error) {
	return _Testcontract.Contract.SucceedOneEvent(&_Testcontract.TransactOpts)
}

// TestcontractEventNoArgsIterator is returned from FilterEventNoArgs and is used to iterate over the raw logs and unpacked data for EventNoArgs events raised by the Testcontract contract.
type TestcontractEventNoArgsIterator struct {
	Event *TestcontractEventNoArgs // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *TestcontractEventNoArgsIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TestcontractEventNoArgs)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(TestcontractEventNoArgs)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *TestcontractEventNoArgsIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TestcontractEventNoArgsIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TestcontractEventNoArgs represents a EventNoArgs event raised by the Testcontract contract.
type TestcontractEventNoArgs struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterEventNoArgs is a free log retrieval operation binding the contract event 0x688e0b89f9b2b23333f4566d66f2cfaf6bfc278bc1c3a26da937fce2c3c8259a.
//
// Solidity: event EventNoArgs()
func (_Testcontract *TestcontractFilterer) FilterEventNoArgs(opts *bind.FilterOpts) (*TestcontractEventNoArgsIterator, error) {

	logs, sub, err := _Testcontract.contract.FilterLogs(opts, "EventNoArgs")
	if err != nil {
		return nil, err
	}
	return &TestcontractEventNoArgsIterator{contract: _Testcontract.contract, event: "EventNoArgs", logs: logs, sub: sub}, nil
}

// WatchEventNoArgs is a free log subscription operation binding the contract event 0x688e0b89f9b2b23333f4566d66f2cfaf6bfc278bc1c3a26da937fce2c3c8259a.
//
// Solidity: event EventNoArgs()
func (_Testcontract *TestcontractFilterer) WatchEventNoArgs(opts *bind.WatchOpts, sink chan<- *TestcontractEventNoArgs) (event.Subscription, error) {

	logs, sub, err := _Testcontract.contract.WatchLogs(opts, "EventNoArgs")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TestcontractEventNoArgs)
				if err := _Testcontract.contract.UnpackLog(event, "EventNoArgs", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseEventNoArgs is a log parse operation binding the contract event 0x688e0b89f9b2b23333f4566d66f2cfaf6bfc278bc1c3a26da937fce2c3c8259a.
//
// Solidity: event EventNoArgs()
func (_Testcontract *TestcontractFilterer) ParseEventNoArgs(log types.Log) (*TestcontractEventNoArgs, error) {
	event := new(TestcontractEventNoArgs)
	if err := _Testcontract.contract.UnpackLog(event, "EventNoArgs", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TestcontractEventOneArgIterator is returned from FilterEventOneArg and is used to iterate over the raw logs and unpacked data for EventOneArg events raised by the Testcontract contract.
type TestcontractEventOneArgIterator struct {
	Event *TestcontractEventOneArg // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *TestcontractEventOneArgIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TestcontractEventOneArg)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(TestcontractEventOneArg)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *TestcontractEventOneArgIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TestcontractEventOneArgIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TestcontractEventOneArg represents a EventOneArg event raised by the Testcontract contract.
type TestcontractEventOneArg struct {
	Number *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterEventOneArg is a free log retrieval operation binding the contract event 0xe59bed36a0b9e33df7f92823525e084566de6674a92ed3c1560b217c5e37967d.
//
// Solidity: event EventOneArg(uint256 number)
func (_Testcontract *TestcontractFilterer) FilterEventOneArg(opts *bind.FilterOpts) (*TestcontractEventOneArgIterator, error) {

	logs, sub, err := _Testcontract.contract.FilterLogs(opts, "EventOneArg")
	if err != nil {
		return nil, err
	}
	return &TestcontractEventOneArgIterator{contract: _Testcontract.contract, event: "EventOneArg", logs: logs, sub: sub}, nil
}

// WatchEventOneArg is a free log subscription operation binding the contract event 0xe59bed36a0b9e33df7f92823525e084566de6674a92ed3c1560b217c5e37967d.
//
// Solidity: event EventOneArg(uint256 number)
func (_Testcontract *TestcontractFilterer) WatchEventOneArg(opts *bind.WatchOpts, sink chan<- *TestcontractEventOneArg) (event.Subscription, error) {

	logs, sub, err := _Testcontract.contract.WatchLogs(opts, "EventOneArg")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TestcontractEventOneArg)
				if err := _Testcontract.contract.UnpackLog(event, "EventOneArg", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseEventOneArg is a log parse operation binding the contract event 0xe59bed36a0b9e33df7f92823525e084566de6674a92ed3c1560b217c5e37967d.
//
// Solidity: event EventOneArg(uint256 number)
func (_Testcontract *TestcontractFilterer) ParseEventOneArg(log types.Log) (*TestcontractEventOneArg, error) {
	event := new(TestcontractEventOneArg)
	if err := _Testcontract.contract.UnpackLog(event, "EventOneArg", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
