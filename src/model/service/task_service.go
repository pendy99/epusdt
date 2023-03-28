package service

import (
	"fmt"
	"math"
	"net/http"
	"sync"

	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/model/request"
	"github.com/assimon/luuu/mq"
	"github.com/assimon/luuu/mq/handle"
	"github.com/assimon/luuu/telegram"
	"github.com/assimon/luuu/util/http_client"
	"github.com/assimon/luuu/util/json"
	"github.com/assimon/luuu/util/log"
	"github.com/golang-module/carbon/v2"
	"github.com/gookit/goutil/stdutil"
	"github.com/hibiken/asynq"
	"github.com/shopspring/decimal"
)

const UsdtTrc20ApiUri = "https://apilist.tronscanapi.com/api/transfer/trc20"
const DebugUsdtTrc20ApiUri = "https://nileapi.tronscan.org/api/transfer/trc20"
const trc20Id = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
const DebugTrc20Id = "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj"

const DebugErc20ApiUrl = "https://api-sepolia.etherscan.io/api"
const erc20ApiUrl = "https://api.etherscan.io/api"
const ercApiKey = "4NVAI1835WTFFD2BDI14QZCET2UGX61N8V"

type UsdtTrc20Resp struct {
	PageSize int    `json:"page_size"`
	Code     int    `json:"code"`
	Data     []Data `json:"data"`
}

type TokenInfo struct {
	TokenID      string `json:"tokenId"`
	TokenAbbr    string `json:"tokenAbbr"`
	TokenName    string `json:"tokenName"`
	TokenDecimal int    `json:"tokenDecimal"`
	TokenCanShow int    `json:"tokenCanShow"`
	TokenType    string `json:"tokenType"`
	TokenLogo    string `json:"tokenLogo"`
	TokenLevel   string `json:"tokenLevel"`
	IssuerAddr   string `json:"issuerAddr"`
	Vip          bool   `json:"vip"`
}

type Data struct {
	Amount         string `json:"amount"`
	ApprovalAmount string `json:"approval_amount"`
	BlockTimestamp int64  `json:"block_timestamp"`
	Block          int    `json:"block"`
	From           string `json:"from"`
	To             string `json:"to"`
	Hash           string `json:"hash"`
	Confirmed      int    `json:"confirmed"`
	ContractType   string `json:"contract_type"`
	ContracTType   int    `json:"contractType"`
	Revert         int    `json:"revert"`
	ContractRet    string `json:"contract_ret"`
	EventType      string `json:"event_type"`
	IssueAddress   string `json:"issue_address"`
	Decimals       int    `json:"decimals"`
	TokenName      string `json:"token_name"`
	ID             string `json:"id"`
	Direction      int    `json:"direction"`
}

type UsdtErc20Resp struct {
	Status string `json:"status"`
	Message string `json:"message"`
	Result []Result `json:"result"`
}
type Result struct {
	BlockNumber int64 `json:"blockNumber"`
	TimeStamp int64 `json:"timeStamp"`
	Hash string `json:"hash"`
	Nonce string `json:"nonce"`
	BlockHash string `json:"blockHash"`
	From string `json:"from"`
	ContractAddress string `json:"contractAddress"`
	To string `json:"to"`
	Value string `json:"value"`
	TokenName string `json:"tokenName"`
	TokenSymbol string `json:"tokenSymbol"`
	TokenDecimal int `json:"tokenDecimal"`
	TransactionIndex int `json:"transactionIndex"`
	Gas string `json:"gas"`
	GasPrice string `json:"gasPrice"`
	GasUsed string `json:"gasUsed"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	Input string `json:"input"`
	Confirmations int64 `json:"confirmations"`
}

// Trc20CallBack trc20回调
func Trc20CallBack(token string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if err := recover(); err != nil {
			log.Sugar.Error(err)
		}
	}()
	token_id := ""
	api_url := ""
	if config.UseTestNet {
		token_id = DebugTrc20Id
		api_url = DebugUsdtTrc20ApiUri
	} else {
		token_id = trc20Id
		api_url = UsdtTrc20ApiUri
	}
	client := http_client.GetHttpClient()
	startTime := carbon.Now().AddHours(-24).TimestampWithMillisecond()
	endTime := carbon.Now().TimestampWithMillisecond()
	resp, err := client.R().SetQueryParams(map[string]string{
		"sort":            "-timestamp",
		"limit":           "50",
		"start":           "0",
		"direction":       "2",
		"db_version":      "1",
		"trc20Id":         token_id,
		"address":         token,
		"start_timestamp": stdutil.ToString(startTime),
		"end_timestamp":   stdutil.ToString(endTime),
	}).Get(api_url)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode() != http.StatusOK {
		panic(err)
	}
	var trc20Resp UsdtTrc20Resp
	err = json.Cjson.Unmarshal(resp.Body(), &trc20Resp)
	if err != nil {
		panic(err)
	}
	if trc20Resp.PageSize <= 0 {
		return
	}
	for _, transfer := range trc20Resp.Data {
		if transfer.To != token || transfer.ContractRet != "SUCCESS" {
			continue
		}
		decimalQuant, err := decimal.NewFromString(transfer.Amount)
		if err != nil {
			panic(err)
		}
		decimalDivisor := decimal.NewFromFloat(1000000)
		amount := decimalQuant.Div(decimalDivisor).InexactFloat64()
		tradeId, err := data.GetTradeIdByWalletAddressAndAmount(token, amount)
		if err != nil {
			panic(err)
		}
		if tradeId == "" {
			continue
		}
		order, err := data.GetOrderInfoByTradeId(tradeId)
		if err != nil {
			panic(err)
		}
		// 区块的确认时间必须在订单创建时间之后
		createTime := order.CreatedAt.TimestampWithMillisecond()
		if transfer.BlockTimestamp < createTime {
			panic("Orders cannot actually be matched")
		}
		// 到这一步就完全算是支付成功了
		req := &request.OrderProcessingRequest{
			Token:              token,
			TradeId:            tradeId,
			Amount:             amount,
			BlockTransactionId: transfer.Hash,
		}
		err = OrderProcessing(req)
		if err != nil {
			panic(err)
		}
		// 回调队列
		orderCallbackQueue, _ := handle.NewOrderCallbackQueue(order)
		mq.MClient.Enqueue(orderCallbackQueue, asynq.MaxRetry(5))
		// 发送机器人消息
		msgTpl := `
<b>📢📢有新的交易支付成功！</b>
<pre>交易号：%s</pre>
<pre>订单号：%s</pre>
<pre>请求支付金额：%f cny</pre>
<pre>实际支付金额：%f usdt</pre>
<pre>钱包地址：%s</pre>
<pre>订单创建时间：%s</pre>
<pre>支付成功时间：%s</pre>
`
		msg := fmt.Sprintf(msgTpl, order.TradeId, order.OrderId, order.Amount, order.ActualAmount, order.Token, order.CreatedAt.ToDateTimeString(), carbon.Now().ToDateTimeString())
		telegram.SendToBot(msg)
	}
}

// Erc20CallBack erc20回调
func Erc20CallBack(token string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if err := recover(); err != nil {
			log.Sugar.Error(err)
		}
	}()
	token_id := ""
	api_url := ""
	if config.UseTestNet {
		token_id = "0x326c977e6efc84e512bb9c30f76e30c160ed06fb"
		api_url = DebugErc20ApiUrl
	} else {
		token_id = "0xdac17f958d2ee523a2206206994597c13d831ec7"
		api_url = erc20ApiUrl
	}
	client := http_client.GetHttpClient()
	resp, err := client.R().SetQueryParams(map[string]string{
		"module":          "account",
		"action":          "tokentx",
		"sort":            "desc",
		"offset":          "100",
		"startblock":      "0",
		"endblock":        "99999999",
		"page":            "1",
		"contractaddress": token_id,
		"address":         token,
		"apikey":          ercApiKey,
	}).Get(api_url)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode() != http.StatusOK {
		panic(err)
	}
	var trc20Resp UsdtErc20Resp
	err = json.Cjson.Unmarshal(resp.Body(), &trc20Resp)
	if err != nil {
		panic(err)
	}
	if trc20Resp.Status != "1" {
		return
	}
	for _, transfer := range trc20Resp.Result {
		if transfer.To != token || transfer.Confirmations < 12 {
			continue
		}
		decimalQuant, err := decimal.NewFromString(transfer.Value)
		if err != nil {
			panic(err)
		}
		decimalDivisor := decimal.NewFromFloat(math.Pow10(transfer.TokenDecimal))
		amount := decimalQuant.Div(decimalDivisor).InexactFloat64()
		tradeId, err := data.GetTradeIdByWalletAddressAndAmount(token, amount)
		if err != nil {
			panic(err)
		}
		if tradeId == "" {
			continue
		}
		order, err := data.GetOrderInfoByTradeId(tradeId)
		if err != nil {
			panic(err)
		}
		// 区块的确认时间必须在订单创建时间之后
		createTime := order.CreatedAt.TimestampWithMillisecond()
		if transfer.TimeStamp < createTime {
			panic("Orders cannot actually be matched")
		}
		// 到这一步就完全算是支付成功了
		req := &request.OrderProcessingRequest{
			Token:              token,
			TradeId:            tradeId,
			Amount:             amount,
			BlockTransactionId: transfer.Hash,
		}
		err = OrderProcessing(req)
		if err != nil {
			panic(err)
		}
		// 回调队列
		orderCallbackQueue, _ := handle.NewOrderCallbackQueue(order)
		mq.MClient.Enqueue(orderCallbackQueue, asynq.MaxRetry(15))
		// 发送机器人消息
		msgTpl := `
<b>📢📢有新的交易支付成功！</b>
<pre>交易号：%s</pre>
<pre>订单号：%s</pre>
<pre>请求支付金额：%f cny</pre>
<pre>实际支付金额：%f usdt</pre>
<pre>钱包地址：%s</pre>
<pre>订单创建时间：%s</pre>
<pre>支付成功时间：%s</pre>
`
		msg := fmt.Sprintf(msgTpl, order.TradeId, order.OrderId, order.Amount, order.ActualAmount, order.Token, order.CreatedAt.ToDateTimeString(), carbon.Now().ToDateTimeString())
		telegram.SendToBot(msg)
	}
}
