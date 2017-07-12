package skyaway

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"errors"

	"github.com/skycoin/skycoin/src/api/webrpc"
	"github.com/skycoin/skycoin/src/coin"
	"github.com/skycoin/skycoin/src/cipher"
	"github.com/skycoin/skycoin/src/visor"
)

type unspentOut struct {
	visor.ReadableOutput
}

type unspentOutSet struct {
	visor.ReadableOutputSet
}

func getUnspent(rpcAddress string, addrs []string) (unspentOutSet, error) {
	req, err := webrpc.NewRequest("get_outputs", addrs, "1")
	if err != nil {
		return unspentOutSet{}, fmt.Errorf("failed to create webrpc request: %v", err)
	}

	rsp, err := webrpc.Do(req, rpcAddress)
	if err != nil {
		return unspentOutSet{}, fmt.Errorf("failed to send rpc request: %v", err)
	}

	if rsp.Error != nil {
		return unspentOutSet{}, fmt.Errorf("rpc error response: %+v", *rsp.Error)
	}

	var rlt webrpc.OutputsResult
	if err := json.Unmarshal(rsp.Result, &rlt); err != nil {
		return unspentOutSet{}, fmt.Errorf("failed to decode rpc response: %v", err)
	}

	return unspentOutSet{rlt.Outputs}, nil
}

func getSufficientUnspents(unspents []unspentOut, amt uint64) ([]unspentOut, error) {
	var (
		totalAmt uint64
		outs     []unspentOut
	)

	addrOuts := make(map[string][]unspentOut)
	for _, u := range unspents {
		addrOuts[u.Address] = append(addrOuts[u.Address], u)
	}

	for _, us := range addrOuts {
		var tmpAmt uint64
		for i, u := range us {
			coins, err := strconv.ParseUint(u.Coins, 10, 64)
			if err != nil {
				return nil, errors.New("error coins string")
			}
			if coins == 0 {
				continue
			}
			tmpAmt = (coins * 1e6)
			us[i].Coins = strconv.FormatUint(tmpAmt, 10)
			totalAmt += coins
			outs = append(outs, us[i])

			if totalAmt >= amt {
				return outs, nil
			}
		}
	}

	return nil, errors.New("balance in wallet is not sufficient")
}

type sendToArg struct {
	Addr  string `json:"addr"`  // send to address
	Coins uint64 `json:"coins"` // send amount
}

func makeChangeOut(outs []unspentOut, chgAddr string, toArgs ...sendToArg) ([]coin.TransactionOutput, error) {
	var (
		totalInAmt   uint64
		totalInHours uint64
		totalOutAmt  uint64
	)

	for _, o := range outs {
		c, err := strconv.ParseUint(o.Coins, 10, 64)
		if err != nil {
			return nil, errors.New("error coins string")
		}
		totalInAmt += c
		totalInHours += o.Hours
	}

	for _, to := range toArgs {
		totalOutAmt += to.Coins
	}

	if totalInAmt < totalOutAmt {
		return nil, errors.New("amount is not sufficient")
	}

	outAddrs := []coin.TransactionOutput{}
	chgAmt := totalInAmt - totalOutAmt*1e6
	chgHours := totalInHours / 4
	addrHours := chgHours / uint64(len(toArgs))
	if chgAmt > 0 {
		// generate a change address
		outAddrs = append(outAddrs, mustMakeUtxoOutput(chgAddr, chgAmt, chgHours/2))
	}

	for _, arg := range toArgs {
		outAddrs = append(outAddrs, mustMakeUtxoOutput(arg.Addr, arg.Coins*1e6, addrHours))
	}

	return outAddrs, nil
}

func mustMakeUtxoOutput(addr string, amount uint64, hours uint64) coin.TransactionOutput {
	uo := coin.TransactionOutput{}
	uo.Address = cipher.MustDecodeBase58Address(addr)
	uo.Coins = amount
	uo.Hours = hours
	return uo
}

func newTransaction(utxos []unspentOut, keys []cipher.SecKey, outs []coin.TransactionOutput) (*coin.Transaction, error) {
	tx := coin.Transaction{}
	for _, u := range utxos {
		tx.PushInput(cipher.MustSHA256FromHex(u.Hash))
	}

	for _, o := range outs {
		if (o.Coins % 1e6) != 0 {
			return nil, errors.New("skycoin coins must be multiple of 1e6")
		}
		tx.PushOutput(o.Address, o.Coins, o.Hours)
	}
	// tx.Verify()

	tx.SignInputs(keys)
	tx.UpdateHeader()
	return &tx, nil
}

func broadcastTx(rpcAddress string, rawtx string) (string, error) {
	params := []string{rawtx}
	req, err := webrpc.NewRequest("inject_transaction", params, "1")
	if err != nil {
		return "", fmt.Errorf("failed to create rpc request: %v", err)
	}

	rsp, err := webrpc.Do(req, rpcAddress)
	if err != nil {
		return "", fmt.Errorf("failed to send rpc request: %v", err)
	}

	if rsp.Error != nil {
		return "", fmt.Errorf("rpc error response: %+v", *rsp.Error)
	}

	var rlt webrpc.TxIDJson
	if err := json.Unmarshal(rsp.Result, &rlt); err != nil {
		return "", fmt.Errorf("failed to decode rpc response")
	}

	return rlt.Txid, nil
}

func (bot *Bot) SendCoins(coins uint64, address string) error {
	unspents, err := getUnspent(bot.config.Wallet.RPC, []string{bot.config.Wallet.Address})
	if err != nil {
		return fmt.Errorf("failed to count unspent coins: %v", err)
	}

	spdouts := unspents.SpendableOutputs()
	spendableOuts := make([]unspentOut, len(spdouts))
	for i := range spdouts {
		spendableOuts[i] = unspentOut{spdouts[i]}
	}

	outs, err := getSufficientUnspents(spendableOuts, coins)
	if err != nil {
		return fmt.Errorf("failed to gather sufficient unspents: %v", err)
	}

	key, err := cipher.SecKeyFromHex(bot.config.Wallet.SecretKey)
	if err != nil {
		return fmt.Errorf("failed to decode the secret key from hex: %v", err)
	}

	chgAddr := bot.config.Wallet.Address // use `from` as the change address
	keys := []cipher.SecKey{key}
	txOuts, err := makeChangeOut(outs, chgAddr, sendToArg{address, coins})
	if err != nil {
		return fmt.Errorf("failed to calculate the change: %v", err)
	}

	tx, err := newTransaction(outs, keys, txOuts)
	if err != nil {
		return fmt.Errorf("failed to create the transaction: %v", err)
	}

	d := tx.Serialize()
	log.Printf("transaction: %s", d)
	rawtx := hex.EncodeToString(d)
	log.Printf("raw transaction: %s", rawtx)
	return errors.New("not implemented")

	txid, err := broadcastTx(bot.config.Wallet.RPC, rawtx)
	log.Printf("txid: %s", txid)
	if err != nil {
		return err
	}

	return nil
}
