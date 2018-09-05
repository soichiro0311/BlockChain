package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo"
)

/*
BlockChain ブロックチェーン
*/
type BlockChain struct {
	// このブロックチェーンに含まれるブロックの配列
	Chain []Block
	// このブロックチェーン上の現在のトランザクション
	CurrentTransaction Transaction
	// このブロックチェーンに接続されている端末の配列
	Nodes []string
}

/*
Transaction トランザクション
*/
type Transaction struct {
	//このトランザクションの送信者
	Sender string
	//このトランザクションの受信者
	Recipient string
	//このトランザクションの取引数量
	Amount int
}

/*
Block ブロック
*/
type Block struct {
	// 所属するブロックチェーン上でのインデックス
	Index int `json:"index"`
	// ブロック生成時のタイムスタンプ
	Timestamp int64 `json:"timestamp"`
	// このブロックに含まれるトランザクション
	Transactions Transaction `json:"transactions"`
	// このブロックに含まれるプルーフ
	Proof int `json:"proof"`
	// このブロックの一つ前のブロックのハッシュ値
	PreviousHash string `json:"previous_hash"`
}

/*
FullChain ブロックチェーンに含まれるチェーン
*/
type FullChain struct {
	// このチェーンを構成するブロックの配列
	Chain []Block `json:"chain"`
	// このチェーンの総数
	Length int `json:"length"`
}

var blockchain BlockChain

func (BlockChain) init() {
	// ジェネシスブロックを作る
	blockchain = BlockChain{}
	blockchain.NewBlock("1", 100)
}

/*
NewBlock ブロックチェーンに新しいブロックを作る
 :param proof: <int> プルーフオブワークアルゴリズムアルゴリズムから得られるプルーフ
 :prama previousHash <str> 前のブロックのハッシュ
 :return <dict> 新しいブロック
*/
func (BlockChain) NewBlock(PreviousHash string, proof int) Block {
	pg := ""
	// 一つ前のブロックのハッシュを取得
	if PreviousHash != "" {
		pg = PreviousHash
		// 一つ前のブロックのハッシュがなかった場合、所属するブロックチェーンの最後のブロックをハッシュ化
	} else {
		pg = blockchain.Hash(blockchain.Chain[len(blockchain.Chain)-1])
	}
	//新しいブロックを作成
	block := Block{
		Index:        len(blockchain.Chain) + 1,
		Timestamp:    time.Now().Unix(),
		Transactions: blockchain.CurrentTransaction,
		Proof:        proof,
		PreviousHash: pg,
	}

	blockchain.CurrentTransaction = Transaction{}
	blockchain.Chain = append(blockchain.Chain, block)
	return block
}

/*
NewTransaction 新しいトランザクションを作成する
 :param sender: <str> トランザクションの送信者
 :prama recipient <str> トランザクションの受信者
 :param amount <int> トランザクションの取引数量

*/
func (BlockChain) NewTransaction(sender string, recipient string, amount int) int {
	blockchain.CurrentTransaction = Transaction{Sender: sender, Recipient: recipient, Amount: amount}
	return blockchain.LastBlock().Index + 1
}

/*
LastBlock ブロックチェーンの最後のブロックを取得する
*/
func (BlockChain) LastBlock() Block {
	return blockchain.Chain[len(blockchain.Chain)-1]
}

/*
Hash ブロックのSHA-256ハッシュを作る
 :param block: <dict> ブロック
 :return <str>
*/
func (BlockChain) Hash(block Block) string {
	blockJson, err := json.Marshal(block)
	if err != nil {
		panic(err)
	}
	converted := sha256.Sum256([]byte(blockJson))
	return hex.EncodeToString(converted[:])
}

/*
ProofOfWork プルーフオブワークを行う
 :param lastProof: <int> ブロックチェーン上の最後のブロックのプルーフ
*/
func (BlockChain) ProofOfWork(lastProof int) int {
	proof := 0
	for blockchain.validProof(lastProof, proof) == false {
		proof++
	}

	return proof
}

/*
validProof ブロックチェーン上の最後のブロックのプルーフと新しいプルーフで検証を行う
 :param lastProof: ブロックチェーン上の最後のブロックのプルーフ
 :param proof: 検証対象のプルーフ
*/
func (BlockChain) validProof(lastProof int, proof int) bool {
	guess := []byte(strconv.Itoa(lastProof) + strconv.Itoa(proof))
	sha256s := sha256.Sum256(guess)
	guessHash := hex.EncodeToString(sha256s[:])
	return guessHash[:4] == "0000"
}

func (BlockChain) validChain(chain []Block) bool {
	lastBlock := chain[0]
	currentIndex := 1

	for currentIndex < len(chain) {
		block := chain[currentIndex]

		if block.PreviousHash != blockchain.Hash(lastBlock) {
			return false
		}

		lastBlock = block
		currentIndex++

	}
	return true
}

func (BlockChain) resolveConflicts() bool {
	neighbors := blockchain.Nodes
	var newChain []Block

	maxLength := len(blockchain.Chain)

	for _, node := range neighbors {
		response, err := http.Get(node + "/chain")
		if err != nil {
			panic(err)
		}

		if response.StatusCode != 200 {
			panic(err)
		}

		var fullChain FullChain
		if err := json.NewDecoder(response.Body).Decode(&fullChain); err != nil {
			panic(err)
		}

		length := fullChain.Length
		chain := fullChain.Chain

		if length > maxLength && blockchain.validChain(chain) {
			maxLength = length
			newChain = chain
		}
	}

	if len(newChain) != 0 {
		blockchain.Chain = newChain
		return true
	}
	return false
}

func (BlockChain) RegisterNode(address string) {
	blockchain.Nodes = append(blockchain.Nodes, address)

	fix := make(map[string]bool)
	one := []string{}
	for _, a := range blockchain.Nodes {
		if !fix[a] {
			fix[a] = true
			one = append(one, a)

		}
	}
	blockchain.Nodes = one
}

var nodeIdentifire string

func main() {
	e := echo.New()

	nodeIdentifire = strings.Replace(uuid.New().String(), "-", "", -1)
	blockchain.init()

	e.GET("/mine", Mine)
	e.POST("/transactions/new", NewTransactionPost)
	e.GET("/chain", FullChainGET)
	e.POST("/nodes/register", RegisterNode)
	e.GET("/nodes/resolve", Consensus)

	go func(echoEcho *echo.Echo) {
		copyEcho := echoEcho
		copyEcho.Start(":5001")
	}(e)
	e.Start(":5000")
}

type Post2 struct {
	Nodes []string `json:"nodes"`
}

type Response2 struct {
	Message   string   `json:"message"`
	TotalNode []string `json:"total_node"`
}

func RegisterNode(e echo.Context) error {
	nodes := new(Post2)
	if err := e.Bind(nodes); err != nil {
		return e.JSON(http.StatusBadRequest, "Status Bad Request.")
	}

	for _, node := range nodes.Nodes {
		blockchain.RegisterNode(node)
	}

	var response2 Response2
	response2.Message = "新しいノードが追加されました"
	response2.TotalNode = blockchain.Nodes

	return e.JSON(http.StatusCreated, response2)
}

func Consensus(e echo.Context) error {
	replaced := blockchain.resolveConflicts()
	if replaced {
		type Response struct {
			Message  string  `json:"message"`
			NewChain []Block `json:"new_chain"`
		}
		var response Response
		response.Message = "チェーンが置き換えられました"
		response.NewChain = blockchain.Chain
		return e.JSON(http.StatusOK, response)
	} else {
		type Response struct {
			Message string  `json:"message"`
			Chain   []Block `json:"chain"`
		}
		var response Response
		response.Message = "チェーンが確認されました"
		response.Chain = blockchain.Chain
		return e.JSON(http.StatusOK, response)
	}
}

type Post struct {
	Sender    string
	Recipient string
	Amount    int
}

func NewTransactionPost(e echo.Context) error {
	post := new(Post)
	if err := e.Bind(post); err != nil {
		return e.JSON(http.StatusBadRequest, "Status Bad Request.")
	}

	index := blockchain.NewTransaction(post.Sender, post.Recipient, post.Amount)
	return e.JSON(http.StatusCreated, "トランザクションはブロック"+strconv.Itoa(index)+"に追加されました")
}

func Mine(e echo.Context) error {
	lastBlock := blockchain.LastBlock()
	lastProof := lastBlock.Proof
	proof := blockchain.ProofOfWork(lastProof)

	blockchain.NewTransaction("0", nodeIdentifire, 1)
	block := blockchain.NewBlock("", proof)

	response := struct {
		Message      string      `json:"Message"`
		Index        int         `json:"index"`
		Transactions Transaction `json:"transactions"`
		Proof        int         `json:"proof"`
		PreviousHash string      `json:"previous_hash"`
	}{
		Message:      "新しいブロックを採掘しました",
		Index:        block.Index,
		Transactions: block.Transactions,
		Proof:        block.Proof,
		PreviousHash: block.PreviousHash,
	}
	return e.JSON(http.StatusCreated, response)
}

func FullChainGET(e echo.Context) error {
	var response FullChain
	response.Chain = blockchain.Chain
	response.Length = len(blockchain.Chain)

	return e.JSON(http.StatusOK, response)
}
