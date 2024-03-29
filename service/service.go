package service

import (
	"encoding/json"
	"errors"
	"bytes"
	"math/big"
	"math"
	"fmt"

	. "github.com/elastos/Elastos.ELA.Utility/common"
	"github.com/elastos/Elastos.ELA.Utility/http/util"

	sideser "github.com/elastos/Elastos.ELA.SideChain/service"
	side "github.com/elastos/Elastos.ELA.SideChain/types"

	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/avm/datatype"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/avm"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/types"
	"github.com/elastos/Elastos.ELA.SideChain.NeoVM/params"
	vmerr "github.com/elastos/Elastos.ELA.SideChain.NeoVM/avm/errors"
)

type HttpServiceExtend struct {
	*sideser.HttpService

	cfg   *sideser.Config
	elaAssetID Uint256
}

func NewHttpService(cfg *sideser.Config, assetid Uint256) *HttpServiceExtend {
	server := &HttpServiceExtend{
		HttpService: sideser.NewHttpService(cfg),
		cfg:         cfg,
		elaAssetID: assetid,
	}
	return server
}

func GetTransactionInfoFromBytes(txInfoBytes []byte) (*sideser.TransactionInfo, error) {
	var txInfo sideser.TransactionInfo
	err := json.Unmarshal(txInfoBytes, &txInfo)
	if err != nil {
		return nil, errors.New("InvalidParameter")
	}

	var assetInfo sideser.PayloadInfo
	switch txInfo.TxType {
	case side.CoinBase:
		assetInfo = &sideser.CoinbaseInfo{}
	case side.RegisterAsset:
		assetInfo = &sideser.RegisterAssetInfo{}
	case side.SideChainPow:
		assetInfo = &sideser.SideChainPowInfo{}
	case side.RechargeToSideChain:
		if txInfo.PayloadVersion == side.RechargeToSideChainPayloadVersion0 {
			assetInfo = &sideser.RechargeToSideChainInfoV0{}
		} else if txInfo.PayloadVersion == side.RechargeToSideChainPayloadVersion1 {
			assetInfo = &sideser.RechargeToSideChainInfoV1{}
		}
	case side.TransferCrossChainAsset:
		assetInfo = &sideser.TransferCrossChainAssetInfo{}
	case side.Deploy:
		assetInfo = &DeployInfo{}
	case side.Invoke:
		assetInfo = &InvokeInfo{}
	default:
		return nil, errors.New("GetBlockTransactions: Unknown payload type")
	}
	err = sideser.Unmarshal(&txInfo.Payload, assetInfo)
	if err == nil {
		txInfo.Payload = assetInfo
	}

	return &txInfo, nil
}

func GetTransactionInfo(cfg *sideser.Config, header *side.Header, tx *side.Transaction) *sideser.TransactionInfo {
	inputs := make([]sideser.InputInfo, len(tx.Inputs))
	for i, v := range tx.Inputs {
		inputs[i].TxID = sideser.ToReversedString(v.Previous.TxID)
		inputs[i].VOut = v.Previous.Index
		inputs[i].Sequence = v.Sequence
	}

	outputs := make([]sideser.OutputInfo, len(tx.Outputs))
	for i, v := range tx.Outputs {
		outputs[i].Value = v.Value.String()
		outputs[i].Index = uint32(i)
		address, _ := v.ProgramHash.ToAddress()
		outputs[i].Address = address
		outputs[i].AssetID = sideser.ToReversedString(v.AssetID)
		outputs[i].OutputLock = v.OutputLock
	}

	attributes := make([]sideser.AttributeInfo, len(tx.Attributes))
	for i, v := range tx.Attributes {
		attributes[i].Usage = v.Usage
		attributes[i].Data = BytesToHexString(v.Data)
	}

	programs := make([]sideser.ProgramInfo, len(tx.Programs))
	for i, v := range tx.Programs {
		programs[i].Code = BytesToHexString(v.Code)
		programs[i].Parameter = BytesToHexString(v.Parameter)
	}

	var txHash = tx.Hash()
	var txHashStr = sideser.ToReversedString(txHash)
	var size = uint32(tx.GetSize())
	var blockHash string
	var confirmations uint32
	var time uint32
	var blockTime uint32
	if header != nil {
		confirmations = cfg.Chain.GetBestHeight() - header.Height + 1
		blockHash = sideser.ToReversedString(header.Hash())
		time = header.Timestamp
		blockTime = header.Timestamp
	}

	return &sideser.TransactionInfo{
		TxId:           txHashStr,
		Hash:           txHashStr,
		Size:           size,
		VSize:          size,
		Version:        0x00,
		LockTime:       tx.LockTime,
		Inputs:         inputs,
		Outputs:        outputs,
		BlockHash:      blockHash,
		Confirmations:  confirmations,
		Time:           time,
		BlockTime:      blockTime,
		TxType:         tx.TxType,
		PayloadVersion: tx.PayloadVersion,
		Payload:        cfg.GetPayloadInfo(tx.Payload, tx.PayloadVersion),
		Attributes:     attributes,
		Programs:       programs,
	}
}

func GetPayloadInfo(p side.Payload, pVersion byte) sideser.PayloadInfo {
	switch object := p.(type) {
	case *side.PayloadCoinBase:
		obj := new(sideser.CoinbaseInfo)
		obj.CoinbaseData = string(object.CoinbaseData)
		return obj
	case *side.PayloadRegisterAsset:
		obj := new(sideser.RegisterAssetInfo)
		obj.Asset = object.Asset
		obj.Amount = object.Amount.String()
		obj.Controller = BytesToHexString(BytesReverse(object.Controller.Bytes()))
		return obj
	case *side.PayloadTransferCrossChainAsset:
		obj := new(sideser.TransferCrossChainAssetInfo)
		obj.CrossChainAssets = make([]sideser.CrossChainAssetInfo, 0)
		for i := 0; i < len(object.CrossChainAddresses); i++ {
			assetInfo := sideser.CrossChainAssetInfo{
				CrossChainAddress: object.CrossChainAddresses[i],
				OutputIndex:       object.OutputIndexes[i],
				CrossChainAmount:  object.CrossChainAmounts[i].String(),
			}
			obj.CrossChainAssets = append(obj.CrossChainAssets, assetInfo)
		}
		return obj
	case *side.PayloadTransferAsset:
	case *side.PayloadRecord:
	case *side.PayloadRechargeToSideChain:
		if pVersion == side.RechargeToSideChainPayloadVersion0 {
			obj := new(sideser.RechargeToSideChainInfoV0)
			obj.MainChainTransaction = BytesToHexString(object.MainChainTransaction)
			obj.Proof = BytesToHexString(object.MerkleProof)
			return obj
		} else if pVersion == side.RechargeToSideChainPayloadVersion1 {
			obj := new(sideser.RechargeToSideChainInfoV1)
			obj.MainChainTransactionHash = sideser.ToReversedString(object.MainChainTransactionHash)
			return obj
		}
	case *types.PayloadDeploy:
		obj := new(DeployInfo)
		obj.Code = *object.Code
		obj.Name = object.Name
		obj.CodeVersion = object.CodeVersion
		obj.Author = object.Author
		obj.Email = object.Email
		obj.Description = object.Description
		obj.ProgramHash = BytesToHexString(BytesReverse(object.ProgramHash.Bytes()))
		obj.Gas = object.Gas.String()
		return obj
	case *types.PayloadInvoke:
		obj := new(InvokeInfo)
		obj.CodeHash = BytesToHexString(BytesReverse(object.CodeHash.Bytes()))
		obj.Code = BytesToHexString(object.Code)
		obj.ProgramHash = BytesToHexString(BytesReverse(object.ProgramHash.Bytes()))
		obj.Gas = object.Gas.String()
		return obj
	}
	return nil
}


func (s *HttpServiceExtend) GetReceivedByAddress(param util.Params) (interface{}, error) {
	tokenValueList := make(map[Uint256]*big.Int)
	var elaValue Fixed64
	str, ok := param.String("address")
	if !ok {
		return nil, fmt.Errorf(sideser.InvalidParams.String())
	}

	programHash, err := Uint168FromAddress(str)
	if err != nil {
		return nil, fmt.Errorf(sideser.InvalidParams.String())
	}

	unspends, err := s.cfg.Chain.GetUnspents(*programHash)
	for assetID, utxos := range unspends {
		for _, u := range utxos {
			if assetID == side.GetSystemAssetId() {
				elaValue += u.Value
			} else {
				data, _ := u.Value.Bytes()
				value := new(big.Int).SetBytes(data)
				if _, ok := tokenValueList[assetID]; !ok {
					tokenValueList[assetID] = new(big.Int)
				}
				tokenValueList[assetID] = tokenValueList[assetID].Add(tokenValueList[assetID], value)
			}
		}
	}
	valueList := make(map[string]string)
	valueList[BytesToHexString(BytesReverse(side.GetSystemAssetId().Bytes()))] = elaValue.String()
	for k, v := range tokenValueList {
		reverse, _ := Uint256FromBytes(BytesReverse(k.Bytes()))
		totalValue, _ := new(big.Int).SetString(v.String(), 10)
		valueList[reverse.String()] = totalValue.Div(totalValue, big.NewInt(int64(math.Pow10(18)))).String()
	}
	if assetID, ok := param.String("assetid"); ok {
		return map[string]string{assetID: valueList[assetID]}, nil
	} else {
		return valueList, nil
	}
}

func (s *HttpServiceExtend) ListUnspent(param util.Params) (interface{}, error) {
	bestHeight := s.cfg.Chain.GetBestHeight()
	var result []UTXOInfo
	addresses, ok := ArrayString(param["addresses"])
	if !ok {
		return nil, util.NewError(int(sideser.InvalidParams), "need addresses in an array!")
	}
	for _, address := range addresses {
		programHash, err := Uint168FromAddress(address)
		if err != nil {
			return nil, util.NewError(int(sideser.InvalidParams), "Invalid address: "+address)
		}
		unspends, err := s.cfg.Chain.GetUnspents(*programHash)
		if err != nil {
			return nil, util.NewError(int(sideser.InvalidParams), "cannot get asset with program")
		}

		unspents := unspends[s.elaAssetID]
		for _, unspent := range unspents {
			_, height, err := s.cfg.Chain.GetTransaction(unspent.TxId)
			if err != nil {
				return nil, util.NewError(int(sideser.InternalError),
					"unknown transaction "+unspent.TxId.String()+" from persisted utxo")
			}

			result = append(result, UTXOInfo{
				AssetId:       sideser.ToReversedString(s.elaAssetID),
				Txid:          sideser.ToReversedString(unspent.TxId),
				VOut:          unspent.Index,
				Amount:        unspent.Value.String(),
				Address:       address,
				Confirmations: bestHeight - height + 1,
			})
		}
	}
	return result, nil
}

func (s *HttpServiceExtend) InvokeScript(param util.Params) (interface{}, error) {
	script, ok := param.String("script")
	if !ok {
		return nil, util.NewError(int(sideser.InvalidParams), "Invalid script: "+ script)
	}
	code, err := HexStringToBytes(script)
	if err != nil {
		return nil, util.NewError(int(sideser.InvalidParams), "script is error hexString")
	}

	returntype, ok:= param.String("returntype")
	if !ok {
		returntype = "Void"
	}

	engine, err := RunScript(code)

	var ret map[string]interface{}
	ret = make(map[string]interface{})
	ret["state"] = engine.GetState()
	ret["descript"] = GetDescByVMState(engine.GetState())
	value := Fixed64(engine.GetGasConsumed())
	ret["gas_consumed"] = value.String()
	if engine.GetEvaluationStack().Count() > 0 {
		ret["result"] = getResult(avm.PopStackItem(engine), returntype)
	}

	return ret, err
}

func (s *HttpServiceExtend) InvokeFunction(param util.Params) (interface{}, error) {
	buffer := new(bytes.Buffer)
	paramBuilder := avm.NewParamsBuider(buffer)

	args, ok := param["params"]
	if ok {
		argsData := args.([]interface{})
		if argsData != nil {
			count := len(argsData)
			for i := count - 1; i >= 0; i-- {
				paraseJsonToBytes(argsData[i].(map[string]interface{}), paramBuilder)
			}
		}
	}
	operation, ok := param.String("operation")
	if ok && operation != "" {
		paramBuilder.EmitPushByteArray([]byte(operation))
	}
	returnType, ok := param.String("returntype")
	if !ok {
		returnType = "Void"
	}

	script, ok := param.String("scripthash")
	if !ok {
		return nil, util.NewError(int(sideser.InvalidParams), "Invalid hex: "+ script)
	}
	codeHashBytes, err := HexStringToBytes(script)
	if err != nil {
		return nil, util.NewError(int(sideser.InvalidParams), "Invalid hex: "+ err.Error())
	}
	codeHash, err := Uint168FromBytes(codeHashBytes)
	if err != nil {
		codeHash = &Uint168{}
	}
	if len(codeHashBytes) == 21 {
		codeHashBytes = params.UInt168ToUInt160(codeHash)
	}
	codeHashBytes = BytesReverse(codeHashBytes)
	paramBuilder.EmitPushCall(codeHashBytes)
	engine, err := RunScript(paramBuilder.Bytes())
	if err != nil {
		return false, nil
	}
	var ret map[string]interface{}
	ret = make(map[string]interface{})
	ret["state"] = engine.GetState()
	ret["descript"] = GetDescByVMState(engine.GetState())
	value := Fixed64(engine.GetGasConsumed())
	ret["gas_consumed"] = value.String()
	if engine.GetEvaluationStack().Count() > 0 {
		ret["result"] = getResult(avm.PopStackItem(engine), returnType)
	}
	return ret, nil
}

func paraseJsonToBytes(item map[string]interface{} , builder *avm.ParamsBuilder) error {
		value := item["value"]
		if item["type"] == "Boolean" {
			builder.EmitPushBool(value.(bool))
		} else if item["type"] == "Integer" {
			value := value.(float64)
			builder.EmitPushInteger(int64(value))
		} else if item["type"] == "String" {
			builder.EmitPushByteArray([]byte(value.(string)))
		} else if item["type"] == "ByteArray" || item["type"] == "Hash256" || item["type"] == "Hash168" {
			paramBytes, err := HexStringToBytes(value.(string))
			if err != nil {
				return errors.New(fmt.Sprint("Invalid param \"", item["type"], "\": ", value))
			}
			builder.EmitPushByteArray(paramBytes)
		} else if item["type"] == "Hash160" {
			paramBytes, err := HexStringToBytes(value.(string))
			if err != nil {
				return errors.New(fmt.Sprint("Invalid param \"", item["type"], "\": ", value))
			}
			if len(paramBytes) == 21 {
				temp := make([]byte, 20)
				copy(temp, paramBytes[1:])
				paramBytes = temp
			}
			builder.EmitPushByteArray(paramBytes)
		} else if item["type"] == "Array" {
			count := len(value.([]interface{}))
			for i := count - 1; i >= 0 ; i-- {
				list := value.([]interface{})
				paraseJsonToBytes(list[i].(map[string]interface{}), builder)
			}

			builder.EmitPushInteger(int64(count))
			builder.Emit(avm.PACK)
		}
	return nil
}

func GetDescByVMState(state avm.VMState) string {
	switch state {
	case avm.FAULT:
		return "contract execution failed。"
	case avm.HALT:
		return "contract execution finished。"
	case avm.BREAK:
		return "contract has breaked。"
	case avm.NONE:
		return "contract execution suc。"
	}
	return "unknown state。"
}

func getResult(item datatype.StackItem, returnType string) interface{} {
	if returnType == "String" {
		return string(item.GetByteArray())
	} else if returnType == "Integer" {
		return item.GetBigInteger().Int64()
	} else if returnType == "Hash168" {
		return BytesToHexString(item.GetByteArray())
	} else if returnType == "Boolean" {
		return item.GetBoolean()
	}

	switch item.(type) {
	case *datatype.Boolean:
		return item.GetBoolean()
	case *datatype.Integer:
		return item.GetBigInteger().Int64()
	case *datatype.ByteArray:
		 return item.GetByteArray()
	case *datatype.GeneralInterface:
		interop := item.GetInterface()
		buf := bytes.NewBuffer([]byte{})
		interop.Serialize(buf)
		return BytesToHexString(buf.Bytes())
	case *datatype.Array:
		items := item.GetArray()
		size := len(items)
		var list = make([]interface{}, size)
		for i := 0; i < size; i++ {
			list[i] = getResult(items[i], returnType)
			return list
		}
	}
	return ""
}

func (s *HttpServiceExtend) GetOpPrice(param util.Params) (interface{}, error) {
	var ret map[string]interface{}
	ret = make(map[string]interface{})

	op, ok := param.String("op")
	if !ok {
		return ret, util.NewError(int(sideser.InvalidParams), "Invalid script: "+ op)
	}
	isSysCall := false
	opcode, err := avm.GetOPCodeByName(op)
	if err != nil && len(op) > 1 {
		isSysCall = true
	} else if err != nil {
		return ret, err
	}

	buffer := new(bytes.Buffer)
	paramBuilder := avm.NewParamsBuider(buffer)

	if isSysCall {
		args := param["args"]
		if op == "Neo.Storage.Put" {
			list, ok := ArrayString(args)
			if !ok || len(list) < 2 {
				return ret, errors.New("Invalid SysCall args")
			}
			paramBuilder.EmitSysCall(op, "", list[0], list[1])

		} else if op == "Neo.Asset.Renew" {
			num, ok := param.Int64("args")
			if !ok {
				return ret, errors.New("Invalid CHECKMULTISIG args")
			}
			paramBuilder.EmitSysCall(op, num)
		} else {
			paramBuilder.EmitSysCall(op)
		}
	} else if opcode == avm.CHECKMULTISIG {
		num, ok := param.Int64("args")
		if !ok {
			return ret, errors.New("Invalid CHECKMULTISIG args")
		}
		paramBuilder.EmitPushInteger(num)
		paramBuilder.Emit(avm.CHECKMULTISIG)
	} else {
		paramBuilder.Emit(opcode)
	}
	engine, err := RunGetPriceScript(paramBuilder.Bytes())
	if err == vmerr.ErrNotSupportSysCall {
		return false, err
	}
	value := Fixed64(engine.GetGasConsumed())
	ret["gas_consumed"] = value.String()
	return ret, nil
}

func ArrayString(value interface{}) ([]string, bool) {
	switch v := value.(type) {
	case []interface{}:
		var arrayString []string
		for _, param := range v {
			paramString, ok := param.(string)
			if !ok {
				return nil, false
			}
			arrayString = append(arrayString, paramString)
		}
		return arrayString, true

	default:
		return nil, false
	}
}
