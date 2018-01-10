package main

import(
	"bytes"
	"crypto/sha256"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"math/big"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
)

const subsidy=10

type Transaction struct{
	ID []byte
	Vin []TXInput
	Vout []TXOutput
}

//Serialize return a serialized Transaction
func (tx Transaction) Serialize() []byte{
	var encoded bytes.Buffer

	enc:=gob.NewEncoder(&encoded)
	err:=enc.Encode(tx)
	if err!=nil{
		log.Panic(err)
	}

	return encoded.Bytes()
}

//DeserializeTransaction deserialzes a transaction
func DeserializeTransaction(data []byte) Transaction{
	var transaction Transaction

	decoder:=gob.NewDecoder(bytes.NewReader(data))
	err:=decoder.Decode(&transaction)
	if err!=nil{
		log.Panic(err)
	}

	return transaction
}


//Hash returns the hash of the Transaction
func (tx *Transaction) Hash()[]byte{
	var hash [32]byte

	txCopy:=*tx
	txCopy.ID=[]byte{}

	hash=sha256.Sum256(txCopy.Serialize())

	return hash[:]
}

func (tx Transaction) IsCoinBase() bool{
	return len(tx.Vin)==1 && len(tx.Vin[0].Txid)==0 && tx.Vin[0].Vout==-1
}

//Sign signs each input of a Transaction
func (tx *Transaction) Sign(privKey ecdsa.PrivateKey,prevTXs map[string]Transaction){
	if tx.IsCoinBase(){
		return
	}

	txCopy:=tx.TrimmedCopy()

	for inID,vin:=range txCopy.Vin{
		prevTx:=prevTXs[hex.EncodeToString(vin.Txid)]
		txCopy.Vin[inID].Signature=nil
		txCopy.Vin[inID].PubKey=prevTx.Vout[vin.Vout].PubKeyHash
		txCopy.ID=txCopy.Hash()
		txCopy.Vin[inID].PubKey=nil

		r,s,err:=ecdsa.Sign(rand.Reader,&privKey,txCopy.ID)
		if err!=nil{
			log.Panic(err)
		}
		signature:=append(r.Bytes(),s.Bytes()...)

		tx.Vin[inID].Signature=signature
	}
}

//Verify verifies signatures of Transaction inputs
func (tx *Transaction) Verify(prevTXs map[string]Transaction)bool{
	if tx.IsCoinBase(){
		return true
	}

	txCopy:=tx.TrimmedCopy()
	curve:=elliptic.P256()

	for inID,vin:=range tx.Vin{
		prevTx:=prevTXs[hex.EncodeToString(vin.Txid)]
		txCopy.Vin[inID].Signature=nil
		txCopy.Vin[inID].PubKey=prevTx.Vout[vin.Vout].PubKeyHash
		txCopy.ID=txCopy.Hash()
		txCopy.Vin[inID].PubKey=nil

		r:=big.Int{}
		s:=big.Int{}
		sigLen:=len(vin.Signature)
		r.SetBytes(vin.Signature[:(sigLen/2)])
		s.SetBytes(vin.Signature[(sigLen/2):])

		x:=big.Int{}
		y:=big.Int{}
		keyLen:=len(vin.PubKey)
		x.SetBytes(vin.PubKey[:(keyLen/2)])
		y.SetBytes(vin.PubKey[(keyLen/2):])

		rawPubKey:=ecdsa.PublicKey{curve,&x,&y}
		if ecdsa.Verify(&rawPubKey,txCopy.ID,&r,&s)==false{
			return false
		}

	}
	return true
}

//TirmmedCopy Copy a Transaction
func (tx *Transaction) TrimmedCopy() Transaction{
	var inputs []TXInput
	var outputs []TXOutput

	for _,vin:=range tx.Vin{
		inputs=append(inputs,TXInput{vin.Txid,vin.Vout,nil,nil})
	}

	for _,vout:=range tx.Vout{
		outputs=append(outputs,TXOutput{vout.Value,vout.PubKeyHash})
	}

	txCopy:=Transaction{tx.ID,inputs,outputs}

	return txCopy
}

//NewCoinbaseTX creates a new coinbase transaction
func NewCoinbaseTX(to,data string) *Transaction {
	if data==""{
		randData:=make([]byte,20)
		_,err:=rand.Read(randData)
		if err!=nil{
			log.Panic(err)
		}

		data=fmt.Sprintf("%x'",randData)
	}

	txin:=TXInput{[]byte{},-1,nil,[]byte(data)}
	//txout:=TXOutput{subsidy,to}
	txout:=NewTXOutput(subsidy,to)
	tx:=Transaction{nil,[]TXInput{txin},[]TXOutput{*txout}}
	tx.ID=tx.Hash()

	return &tx
}

//NewUTXOTransaction creates a new transaction
func NewUTXOTransaction(wallet *Wallet,to string,amount int,UTXOSet *UTXOSet) *Transaction{
	var inputs []TXInput
	var outputs []TXOutput

	pubKeyHash:=HashPubKey(wallet.PublicKey	)	
	acc,validOutputs:=UTXOSet.FindSpendableOutputs(pubKeyHash,amount)

	if acc<amount{
		log.Panic("ERROR: Not enough funds")
	}

	for txid,outs:=range validOutputs{
		txID,err:=hex.DecodeString(txid)
		if err!=nil{
			log.Panic(err)
		}

		for _,out:=range outs{
			input:=TXInput{txID,out,nil,wallet.PublicKey}
			inputs=append(inputs,input)
		}
	}

	from:=fmt.Sprintf("%s",wallet.GetAddress())
	outputs=append(outputs,*NewTXOutput(amount,to))
	if acc>amount{
		outputs=append(outputs,*NewTXOutput(acc-amount,from))
	}

	tx:=Transaction{nil,inputs,outputs}
	tx.ID=tx.Hash()
	UTXOSet.Blockchain.SignTransaction(&tx,wallet.PrivateKey)	

	return &tx
}
