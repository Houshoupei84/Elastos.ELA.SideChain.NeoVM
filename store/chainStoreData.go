package store

import (
	"errors"
	"bytes"
	"math/big"
	"fmt"

	"github.com/elastos/Elastos.ELA.SideChain/database"
	sb "github.com/elastos/Elastos.ELA.SideChain/blockchain"
	side "github.com/elastos/Elastos.ELA.SideChain/types"

	"github.com/elastos/Elastos.ELA.Utility/common"

	"github.com/elastos/Elastos.ELA.SideChain/events"

	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/contract/states"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/params"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/types"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/smartcontract"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/smartcontract/service"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/avm"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/blockchain"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/event"
)

const AccountPersisFlag = "AccountPersisFlag"

const DEPLOY_TRANSACTION = "DeployTransaction"
const INVOKE_TRANSACTION = "InvokeTransaction"
const RunTime_Notify = "RunTime_Notify"
const RunTime_Log = "RunTime_Log"

// Response represent the response data structure.
type ResponseExt struct {
	Action   string
	Result   bool
	Error    int
	Desc     interface{}
	TxID     string
	CodeHash string
}

func (c *LedgerStore) PersisAccount(batch database.Batch, block *side.Block) error {
	accounts := make(map[common.Uint168]*states.AccountState, 0)
	for _, txn := range block.Transactions {
		for index := 0; index < len(txn.Outputs); index++ {
			output := txn.Outputs[index]
			programHash := output.ProgramHash
			assetId := output.AssetID
			if account, ok := accounts[programHash]; ok {
				account.Balances[assetId] += output.Value
			} else {
				account, err := c.GetAccount(&programHash)
				if err != nil && err.Error() != ErrDBNotFound.Error() {
					return err
				}
				if account != nil {
					account.Balances[assetId] += output.Value
				} else {
					balances := make(map[common.Uint256]common.Fixed64, 0)
					balances[assetId] = output.Value
					account = states.NewAccountState(programHash, balances)
				}
				accounts[programHash] = account
			}
		}

		for index := 0; index < len(txn.Inputs); index++ {
			if txn.TxType == side.CoinBase {
				continue
			}
			input := txn.Inputs[index]
			transaction, _, err := c.GetTransaction(input.Previous.TxID)
			if err != nil {
				return err
			}
			index := input.Previous.Index
			output := transaction.Outputs[index]
			programHash := output.ProgramHash
			assetID := output.AssetID
			if account, ok := accounts[programHash]; ok {
				account.Balances[assetID] -= output.Value
			} else {
				account, err := c.GetAccount(&programHash)
				if err != nil && err.Error() != ErrDBNotFound.Error() {
					return err
				}
				account.Balances[assetID] -= output.Value
				accounts[programHash] = account
			}
			if accounts[programHash].Balances[assetID] < 0 {
				return errors.New(fmt.Sprintf("account programHash:%v, assetId:%v insufficient of balance", programHash, assetID))
			}
		}
	}
	for programHash, value := range accounts {
		accountKey := new(bytes.Buffer)
		accountKey.WriteByte(byte(sb.ST_Account))
		programHash.Serialize(accountKey)

		accountValue := new(bytes.Buffer)
		value.Serialize(accountValue)
		batch.Put(accountKey.Bytes(), accountValue.Bytes())
	}
	return nil
}

func (c *LedgerStore) PersistDeployTransaction(block *side.Block, tx *side.Transaction, batch database.Batch) error {
	payloadDeploy, ok := tx.Payload.(*types.PayloadDeploy)
	if !ok {
		return errors.New("invalid deploy payload")
	}
	dbCache := blockchain.NewDBCache(c)
	smartcontract, err := smartcontract.NewSmartContract(&smartcontract.Context{
		Caller:       payloadDeploy.ProgramHash,
		StateMachine: *service.NewStateMachine(dbCache, dbCache),
		DBCache:      batch,
		Code:         payloadDeploy.Code.Code,
		Time:         big.NewInt(int64(block.Timestamp)),
		BlockNumber:  big.NewInt(int64(block.Height)),
		Gas:          payloadDeploy.Gas,
		Trigger:      avm.Application,
	})
	if err != nil {
		events.Notify(event.ETDeployTransaction, &ResponseExt{
			Action:   DEPLOY_TRANSACTION,
			Result:   false,
			Desc:     err.Error(),
			TxID:     tx.Hash().String(),
			CodeHash: "",
		})
		return err
	}
	ret, err := smartcontract.DeployContract(payloadDeploy)
	if err != nil {
		events.Notify(event.ETDeployTransaction, &ResponseExt{
			Action:   DEPLOY_TRANSACTION,
			Result:   false,
			Desc:     err.Error(),
			TxID:     tx.Hash().String(),
			CodeHash: "",
		})
		return err
	}
	codeHash, err := params.ToCodeHash(ret)
	if err != nil {
		events.Notify(event.ETDeployTransaction, &ResponseExt{
			Action:   DEPLOY_TRANSACTION,
			Result:   false,
			Desc:     err.Error(),
			TxID:     tx.Hash().String(),
			CodeHash: codeHash.String(),
		})
		return err
	}

	contract, err := c.GetContract(codeHash)
	if err != nil && contract != nil {
		events.Notify(event.ETDeployTransaction, &ResponseExt{
			Action:   DEPLOY_TRANSACTION,
			Result:   false,
			Desc:     err.Error(),
			TxID:     tx.Hash().String(),
			CodeHash: codeHash.String(),
		})
		return err
	}
	//because neo compiler use [AppCall(hash)] ，will change hash168 to hash160,so we deploy contract use hash160
	data := params.UInt168ToUInt160(codeHash)

	dbCache.GetOrAdd(sb.ST_Contract, string(data), &states.ContractState{
		Code:        payloadDeploy.Code,
		Name:        payloadDeploy.Name,
		Version:     payloadDeploy.CodeVersion,
		Author:      payloadDeploy.Author,
		Email:       payloadDeploy.Email,
		Description: payloadDeploy.Description,
		ProgramHash: payloadDeploy.ProgramHash,
	})
	log.Info("deploy contract suc:", codeHash.String())
	events.Notify(event.ETDeployTransaction, &ResponseExt{
		Action:   DEPLOY_TRANSACTION,
		Result:   true,
		Desc:     "Success",
		TxID:     tx.Hash().String(),
		CodeHash: codeHash.String(),
	})
	dbCache.Commit()
	return nil
}

func (c *LedgerStore) persisInvokeTransaction(block *side.Block, tx *side.Transaction, batch database.Batch) error {
	payloadInvoke := tx.Payload.(*types.PayloadInvoke)
	constractState := states.NewContractState()
	if !payloadInvoke.CodeHash.IsEqual(common.Uint168{}) {
		contract, err := c.GetContract(&payloadInvoke.CodeHash)
		if err != nil {
			events.Notify(event.ETInvokeTransaction, &ResponseExt{
				Action:   INVOKE_TRANSACTION,
				Result:   false,
				Desc:     err.Error(),
				TxID:     tx.Hash().String(),
				CodeHash: payloadInvoke.CodeHash.String(),
			})
			log.Errorf("invoke transaction failed, txid:%s, error:%s", tx.Hash(), err.Error())
			return err
		}
		state, err := states.GetStateValue(sb.ST_Contract, contract)
		if err != nil {
			events.Notify(event.ETInvokeTransaction, &ResponseExt{
				Action:   INVOKE_TRANSACTION,
				Result:   false,
				Desc:     err.Error(),
				TxID:     tx.Hash().String(),
				CodeHash: payloadInvoke.CodeHash.String(),
			})
			log.Errorf("invoke transaction failed, txid:%s, error:%s", tx.Hash(), err.Error())
			return err
		}
		constractState = state.(*states.ContractState)
	}
	dbCache := blockchain.NewDBCache(c)
	stateMachine := service.NewStateMachine(dbCache, dbCache)
	smartcontract, err := smartcontract.NewSmartContract(&smartcontract.Context{
		Caller:         payloadInvoke.ProgramHash,
		StateMachine:   *stateMachine,
		DBCache:        batch,
		CodeHash:       payloadInvoke.CodeHash,
		Input:          payloadInvoke.Code,
		SignableData:   tx,
		CacheCodeTable: NewCacheCodeTable(dbCache),
		Time:           big.NewInt(int64(block.Timestamp)),
		BlockNumber:    big.NewInt(int64(block.Height)),
		Gas:            payloadInvoke.Gas,
		ReturnType:     constractState.Code.ReturnType,
		ParameterTypes: constractState.Code.ParameterTypes,
		Trigger:        avm.Application,
	})
	if err != nil {
		events.Notify(event.ETInvokeTransaction, &ResponseExt{
			Action:   INVOKE_TRANSACTION,
			Result:   false,
			Desc:     err.Error(),
			TxID:     tx.Hash().String(),
			CodeHash: payloadInvoke.CodeHash.String(),
		})
		log.Errorf("invoke transaction failed, txid:%s, error:%s", tx.Hash(), err.Error())
	}

	ret, err := smartcontract.InvokeContract()
	if err != nil {
		events.Notify(event.ETInvokeTransaction, &ResponseExt{
			Action:   INVOKE_TRANSACTION,
			Result:   false,
			Desc:     err.Error(),
			TxID:     tx.Hash().String(),
			CodeHash: payloadInvoke.CodeHash.String(),
		})
		log.Errorf("invoke transaction failed, txid:%s, error:%s", tx.Hash(), err.Error())
		return err
	}
	log.Info("InvokeContract ret=", ret)
	stateMachine.CloneCache.Commit()
	dbCache.Commit()
	events.Notify(event.ETInvokeTransaction, &ResponseExt{
		Action:   INVOKE_TRANSACTION,
		Result:   true,
		Desc:     ret,
		TxID:     tx.Hash().String(),
		CodeHash: payloadInvoke.CodeHash.String(),
	})

	return nil
}

func (c *LedgerStore) GetContract(codeHash *common.Uint168) ([]byte, error) {
	prefix := []byte{byte(sb.ST_Contract)}

	hashBytes := params.UInt168ToUInt160(codeHash)
	bData, err_get := c.Get(append(prefix, hashBytes...))
	if err_get != nil {
		return nil, err_get
	}
	return bData, nil
}

func (c *LedgerStore) GetAccount(programHash *common.Uint168) (*states.AccountState, error) {
	accountPrefix := []byte{byte(sb.ST_Account)}
	state, err := c.Get(append(accountPrefix, programHash.Bytes()...))
	if err != nil {
		return nil, err
	}

	accountState := new(states.AccountState)
	accountState.Deserialize(bytes.NewBuffer(state))

	return accountState, nil
}

func (c *LedgerStore) Close() error {
	c.Close()
	return nil
}
