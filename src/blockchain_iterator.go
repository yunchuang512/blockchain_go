package main

import(
	"log"
	"github.com/boltdb/bolt"
)

//BlockchainIterator is used to iterate over blockchain blocks
type BlockchainIterator struct{
	currentHash []byte
	db *bolt.DB
}

//Iterator return a iterator of Blockchain
func (bc *Blockchain) Iterator() *BlockchainIterator{
	bci:=&BlockchainIterator{bc.tip,bc.db}

	return bci
}

//Next find next block of Blockchain
func (i *BlockchainIterator)Next()*Block{
	var block *Block
	err:=i.db.View(func(tx *bolt.Tx)error{
		b:=tx.Bucket([]byte(blocksBucket))
		encodedBlock:=b.Get(i.currentHash)
		block=DeserializeBlock(encodedBlock)

		return nil
	})

	if err!=nil{
		log.Panic(err)
	}

	i.currentHash=block.PrevBlockHash

	return block
}