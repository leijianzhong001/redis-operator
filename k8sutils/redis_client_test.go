package k8sutils

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"
)

// 连接单节点
func TestConnectStandalone(t *testing.T) {
	var client *redis.Client
	client = redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "123", // 如果没有密码，这里直接给""即可
		DB:       0,
	})

	ret, err := client.Set(ctx, "k1", "v1", time.Second*10).Result()
	if err != nil {
		t.Errorf("执行set命令报错, %v", err)
		return
	}
	fmt.Println("set命令执行成功，结果:", ret)

	ret, err = client.Get(ctx, "k1").Result()
	if err != nil {
		t.Errorf("执行get命令报错, %v", err)
		return
	}
	fmt.Println("get命令执行成功，结果:", ret)
}

// 通过Sentinel集群连接节点
func TestConnectStandaloneViaSentinel(t *testing.T) {
	var client *redis.Client
	client = redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    "SNRS_SIT_1_36379",
		SentinelAddrs: []string{"10.243.66.12:26380", "10.243.66.12:26381", "10.243.66.12:26382"},
	})

	ret, err := client.Set(ctx, "k1", "v1", time.Second*10).Result()
	if err != nil {
		t.Errorf("执行set命令报错, %v", err)
		return
	}
	fmt.Println("set命令执行成功，结果:", ret)

	ret, err = client.Get(ctx, "k1").Result()
	if err != nil {
		t.Errorf("执行get命令报错, %v", err)
		return
	}
	fmt.Println("get命令执行成功，结果:", ret)
}

// 连接sentinel
func TestConnectToSentinel(t *testing.T) {
	sentinelClient := redis.NewSentinelClient(&redis.Options{
		Addr: "10.243.66.12:26380",
	})

	ip, err := sentinelClient.GetMasterAddrByName(ctx, "SNRS_SIT_1_36379").Result()
	if err != nil {
		t.Errorf("执行GetMasterAddrByName命令报错, %v", err)
		return
	}

	fmt.Println("获取到的ip信息为:", ip)

	client := redis.NewClient(&redis.Options{Addr: "10.243.66.12:26380"})
	result, err := client.Info(ctx, "sentinel").Result()
	if err != nil {
		t.Errorf("执行info sentinel命令报错, %v", err)
		return
	}
	fmt.Println("获取到的shard列表为:", result)
}

// 连接cluster集群
func TestConnectToCluster(t *testing.T) {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        []string{"10.243.66.12:30001", "10.243.66.12:30002", "10.243.66.12:30003"},
		MaxRedirects: 5,
	})

	ret, err := client.Set(ctx, "k1", "v1", time.Second*10).Result()
	if err != nil {
		t.Errorf("执行set命令报错, %v", err)
		return
	}
	fmt.Println("set命令执行成功，结果:", ret)

	ret, err = client.Get(ctx, "k1").Result()
	if err != nil {
		t.Errorf("执行get命令报错, %v", err)
		return
	}
	fmt.Println("get命令执行成功，结果:", ret)

	ret, err = client.ClusterInfo(ctx).Result()
	if err != nil {
		t.Errorf("执行cluster info命令报错, %v", err)
		return
	}
	fmt.Println("cluster info命令执行成功，结果:", ret)

	ret, err = client.ClusterNodes(ctx).Result()
	if err != nil {
		t.Errorf("执行cluster nodes命令报错, %v", err)
		return
	}
	fmt.Println("cluster nodes命令执行成功，结果:\n", ret)

	err = client.ForEachShard(ctx, func(ctx context.Context, shardClient *redis.Client) error {
		result, err := shardClient.ConfigGet(ctx, "appendonly").Result()
		if err != nil {
			return err
		}

		clientId, err := shardClient.ClientID(ctx).Result()
		if err != nil {
			return err
		}
		fmt.Printf("clientId: %d Config Get命令执行成功，结果:%s \n", clientId, result)
		return nil
	})

	if err != nil {
		t.Errorf("执行ForEachShard命令报错, %v", err)
		return
	}

	_ = client.ForEachMaster(ctx, func(ctx context.Context, masterClient *redis.Client) error {
		myId, _ := masterClient.Do(ctx, "cluster", "myid").Text()
		role, _ := masterClient.Do(ctx, "role").Result()
		fmt.Printf("myId: %s  role:%s \n", myId, role)
		return nil
	})

	_ = client.ForEachSlave(ctx, func(ctx context.Context, slaveClient *redis.Client) error {
		myId, _ := slaveClient.Do(ctx, "cluster", "myid").Text()
		role, _ := slaveClient.Do(ctx, "role").Result()
		fmt.Printf("myId: %s  role:%s \n", myId, role)
		return nil
	})
}

// 连接单个的redis cluster节点
func TestConnectClusterNode(t *testing.T) {
	var client *redis.Client
	client = redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:30001",
		Password: "",
		DB:       0,
	})

	result, err := client.Info(ctx).Result()
	if err != nil {
		return
	}

	fmt.Println(result)
}

func TestConnectClusterNodeWithPass(t *testing.T) {
	ctx := context.Background()
	var client *redis.Client
	client = redis.NewClient(&redis.Options{
		Addr:     "10.243.66.13:30001",
		Username: "default",
		Password: "c4b883c1cba107078b6e0eb6f5677b6a4fcf4046639f2d89a5ec43620efe6e12",
		DB:       0,
	})

	result, err := client.Info(ctx).Result()
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	fmt.Println(result)
}

// 扫描并删除
func TestCreateData(t *testing.T) {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        []string{"10.243.66.12:30001", "10.243.66.12:30002", "10.243.66.12:30003"},
		MaxRedirects: 5,
	})

	for i := 1; i <= 30000; i++ {
		client.Do(ctx, "set", "snrs:"+fmt.Sprintf("%d", i), "value:"+fmt.Sprintf("%d", i))
	}
	fmt.Println("Done")
}

func TestStandaloneData(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	for i := 1; i <= 30000; i++ {
		client.Set(ctx, "snrs:"+fmt.Sprintf("%d", i), "value:"+fmt.Sprintf("%d", i), redis.KeepTTL)
	}
	fmt.Println("Done")
}

// 扫描并删除
func TestScanAndDelete(t *testing.T) {

	var client *redis.Client
	client = redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:30003",
		Password: "", // 如果没有密码，这里直接给""即可
		DB:       0,
	})

	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = client.Scan(ctx, cursor, "snrs:*", 50).Result()
		if err != nil {
			panic(err)
		}

		pipeline := client.Pipeline()
		for _, key := range keys {
			pipeline.Unlink(ctx, key)
		}

		exec, err := pipeline.Exec(ctx)
		if err != nil {
			panic(err)
		}
		fmt.Println(exec)
		fmt.Println()

		if cursor == 0 { // no more keys
			break
		}
	}

	fmt.Println("看一下还有没有残留。。。")

	iterator := client.Scan(ctx, 0, "", 0).Iterator()
	for iterator.Next(ctx) {
		fmt.Println(iterator.Val())
	}

	if err := iterator.Err(); err != nil {
		panic(err)
	}
}

func TestScanAndDelete2(t *testing.T) {
	var client *redis.Client
	client = redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:30001",
		Password: "", // 如果没有密码，这里直接给""即可
		DB:       0,
	})

	var cursor uint64
	var keys []string
	var err error
	keys, cursor, err = client.Scan(ctx, cursor, "snrs:*", 50).Result()
	if err != nil {
		panic(err)
	}
	fmt.Println(keys)
	// F:/GO_PATH/pkg/mod/github.com/go-redis/redis/v8@v8.11.5/internal/hashtag/hashtag.go:1
	for _, key := range keys {
		slot := Slot(key)
		fmt.Printf("key: %s, slot: %d\n", key, slot)
	}

	result, err := client.ClusterSlots(ctx).Result()
	if err != nil {
		panic(err)
	}
	fmt.Println(result)

	result2, err := client.ClusterNodes(ctx).Result()
	if err != nil {
		panic(err)
	}
	fmt.Println(result2)

	//result, err := client.Unlink(ctx, keys...).Result()
	//if err != nil {
	//	panic(err)
	//}
	//fmt.Println("unlink 结果：", result)
}

func TestSliceTest(t *testing.T) {
	myMap := make(map[string][]string)
	// 这样make出来的mySlice本身就是一个指针
	mySlice := make([]string, 0, 5)
	fmt.Println("mySlice len", len(mySlice))
	fmt.Println("mySlice cap", cap(mySlice))
	myMap["key1"] = mySlice
	fmt.Printf("before append mySlice %p\n", mySlice)
	fmt.Printf("before append myMap[key1] %p\n", myMap["key1"])

	fmt.Println()

	mySlice = append(mySlice, "a")
	fmt.Printf("after append mySlice %p\n", mySlice)
	mySlice = append(mySlice, "a")
	fmt.Printf("after append mySlice %p\n", mySlice)
	mySlice = append(mySlice, "a")
	fmt.Printf("after append mySlice %p\n", mySlice)

	fmt.Printf("after append myMap[key1] %p\n", myMap["key1"])
	fmt.Println("after append myMap[key1] ", myMap["key1"])
	fmt.Println("mySlice", mySlice)

	fmt.Println()

	println(mySlice)       // [3/5]0xc000399d10
	println(myMap["key1"]) // [0/5]0xc000399d10

	myMap["key1"] = append(myMap["key1"], "aaa")
	fmt.Println(mySlice)

	mySlice[0] = "bbb"
	fmt.Println(myMap)
}

func TestSliceTest2(t *testing.T) {
	book1 := Book{
		Id:      "",
		Name:    "",
		Authors: nil,
		Press:   "",
	}
	book2 := book1
	fmt.Printf("book1 %p\n", &book1)
	fmt.Printf("book2 %p\n", &book2)
}

func TestSliceTest3(t *testing.T) {
	book1 := Book{Name: "战争与和平"}
	book2 := book1
	fmt.Printf("book1 %p\n", &book1)
	fmt.Printf("book2 %p\n", &book2)

	book1_hdr := (*reflect.StringHeader)(unsafe.Pointer(&book1.Name)) // 将string类型变量地址显式转型为reflect.StringHeader
	fmt.Printf("book1.Name.data 0x%x\n", book1_hdr.Data)              // 0x10a30e0
	book1_hdr_p := (*[5]byte)(unsafe.Pointer(book1_hdr.Data))         // 获取Data字段所指向的数组的指针
	fmt.Printf("book1.Name.data %p\n", book1_hdr_p)

	fmt.Println()

	book2_hdr := (*reflect.StringHeader)(unsafe.Pointer(&book2.Name)) // 将string类型变量地址显式转型为reflect.StringHeader
	fmt.Printf("book2.Name.data 0x%x\n", book2_hdr.Data)              // 0x10a30e0
	book2_hdr_p := (*[5]byte)(unsafe.Pointer(book2_hdr.Data))         // 获取Data字段所指向的数组的指针
	fmt.Printf("book2.Name.data %p\n", book2_hdr_p)
}

func TestSliceTest4(t *testing.T) {
	book1 := Book{
		Id:      "ISBN 978-7-115-40284-4",
		Name:    "Redis 实战",
		Authors: []string{"aaa", "bbb", "ccc"},
		Press:   "adfds",
		Labels:  [5]string{"a", "b", "c", "d", "e"},
	}

	book2 := book1

	// 两个对象地址不一样，说明是两个完全独立的对象
	fmt.Printf("book1 %p\n", &book1) // 0xc0003dbd60
	fmt.Printf("book2 %p\n", &book2) // 0xc0003dbdb0

	// 两个切片对象地址不一样，说明是两个完全独立的对象
	fmt.Printf("book1.Authors %p\n", &book1.Authors) // 0xc0003dbd80
	fmt.Printf("book2.Authors %p\n", &book2.Authors) // 0xc0003dbdd0

	// 输出两个切片对象底层数组地址，发现是一样的，说明他们共享底层数组
	fmt.Printf("book1.Authors %p\n", book1.Authors) // 0xc00044e600
	fmt.Printf("book2.Authors %p\n", book2.Authors) // 0xc00044e600

	// 数组拷贝是值拷贝，所以这两个数组对象的地址不同
	fmt.Printf("book1.Labels %p\n", &book1.Labels) // 0xc0004445e8
	fmt.Printf("book2.Labels %p\n", &book2.Labels) // 0xc000444688
	book1.Labels[0] = "aaa"
	fmt.Println(book1.Labels[0])
	fmt.Println(book2.Labels[0])
}

func TestSliceTest5(t *testing.T) {
	mySlice := make([]string, 0, 50)
	mySlice = append(mySlice, "a")
	mySlice = append(mySlice, "b")
	mySlice = append(mySlice, "c")
	// len: 3, cap: 50, ptr: 0xc0002d0700
	fmt.Printf("len: %d, cap: %d, ptr: %p\n", len(mySlice), cap(mySlice), mySlice)

	mySlice2 := mySlice[0:0]

	// len: 0, cap: 50, ptr: 0xc0002d0700
	fmt.Printf("len: %d, cap: %d, ptr: %p\n", len(mySlice2), cap(mySlice2), mySlice2)
}

func TestMap(t *testing.T) {
	myMap := make(map[int][]string, 10)
	myMap[1] = []string{"a", "b", "c"}
	myMap[2] = []string{"a", "b", "c"}
	myMap[3] = []string{"a", "b", "c"}

	for slot, keys := range myMap {
		fmt.Printf("slot: %d, keys: %v\n", slot, keys)
	}
}

type Book struct {
	Id      string    `json:"id"`      // 图书ISBN ID
	Name    string    `json:"name"`    // 图书名称
	Authors []string  `json:"authors"` // 图书作者
	Press   string    `json:"press"`   // 出版社
	Labels  [5]string `json:"labels"`  // 标签
}

func dumpBytesArray(arr []byte) {
	fmt.Printf("[")
	for _, b := range arr {
		fmt.Printf("%c ", b)
	}
	fmt.Printf("]\n")
}

func TestUnsupportedCommands(t *testing.T) {
	var client *redis.Client
	client = redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:30001",
		Password: "", // 如果没有密码，这里直接给""即可
		DB:       0,
	})

	result, err := client.Do(ctx, "cluster", "nodes").Result()
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
	fmt.Printf("%T\n", result)
	str := fmt.Sprintf("%v", result)
	fmt.Println(str)
}

var streamName = "leijianzhong"
var values = map[string]string{"name": "leijianzhong", "age": "30"}
var xAddArgs = &redis.XAddArgs{
	Stream: streamName,
	ID:     "*", // 自动生成id
	Values: values,
}

func TestStreamAPI(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})
	msgIds := make([]string, 0, 30)
	for i := 1; i <= 30; i++ {
		// 不管有多少个键值对，xadd一次算一条消息
		result, err := client.XAdd(ctx, xAddArgs).Result()
		if err != nil {
			panic(err)
		}
		msgIds = append(msgIds, result)
		fmt.Println("XAdd 消息：", result)
	}

	fmt.Println("XRange 获取消息列表(自动忽略以xdel的消息)：", client.XRange(ctx, streamName, "-", "+"))
	fmt.Println("XDel 删除一条消息：", client.XDel(ctx, streamName, msgIds[rand.Intn(30)]))
	fmt.Println("XLen 消息列表长度：", client.XLen(ctx, streamName))
	fmt.Println("Done")
}

// 我们可以在不定义消费组的情况下进行 Stream 消息的独立消费，当 Stream 没有新消息时，甚至可以阻塞等待。
// Redis 设计了一个单独的消费指令xread，可以将 Stream 当成普通的消息队列 (list) 来使用。
// 使用 xread 时，我们可以完全忽略消费组 (Consumer Group) 的存在，就好比 Stream 就是一个普通的列表 (list)。
func TestStreamXRead(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})
	client.Del(ctx, streamName)
	for i := 1; i <= 30; i++ {
		client.XAdd(ctx, xAddArgs)
	}

	// 0-0 从 Stream 头部开始读取消息
	result, err := client.Do(ctx, "xread", "count", "2", "streams", streamName, "0-0").Result()
	if err != nil {
		panic(err)
	}
	fmt.Println("从头部获取两条消息: ", result)

	// 从尾部开始就是从最后一条消息开始，但是不包含在以后一条消息，最后一条消息再往后就没有消息了，除非阻塞等待新的消息到来
	result, err = client.Do(ctx, "xread", "count", "2", "streams", streamName, "$").Result()
	fmt.Println("从尾部开始获取两条消息: ", result)

	// 阻塞等待 block 0一直阻塞
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		// xread count 1 block 0 streams leijianzhong $
		result, _ := client.XRead(ctx, &redis.XReadArgs{
			Streams: []string{streamName, "$"}, // list of streams and ids, e.g. stream1 stream2 id1 id2
			Count:   1,
			Block:   0,
		}).Result()
		fmt.Println("阻塞直到从尾部开始获取1条消息: ", result)
		wg.Done()
	}()

	// 先睡1妙，让上面的xread阻塞操作先于下面的XAdd执行，否则还是会读不到消息
	time.Sleep(time.Second)

	result, _ = client.XAdd(ctx, xAddArgs).Result()
	fmt.Println("添加一条消息到尾部", result)

	wg.Wait()
}

// XInfoStream 获取stream基本信息
func TestStreamXInfoStream(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	// 创建消费者组，从头开始消费. start: 要传递起始消息 ID 参数用来初始化last_delivered_id变量
	client.XGroupCreate(ctx, streamName, "cg1", "0-0")

	// 创建消费者组，从尾部开始消费，只接收新消息.  start: 要传递起始消息 ID 参数用来初始化last_delivered_id变量
	client.XGroupCreate(ctx, streamName, "cg2", "$")

	streamInfo, _ := client.XInfoStream(ctx, streamName).Result()
	fmt.Println("XInfoStream 获取 Stream 信息: ", streamInfo)
	fmt.Println("stream消息数量: ", streamInfo.Length)
	fmt.Println("RadixTreeKeys 数量: ", streamInfo.RadixTreeKeys)
	fmt.Println("RadixTreeNodes 数量: ", streamInfo.RadixTreeNodes)
	fmt.Println("第一条消息: ", streamInfo.FirstEntry)
	fmt.Println("最后一条消息: ", streamInfo.LastEntry)
	fmt.Println("消费者组数量: ", streamInfo.Groups)
}

// TestStreamXInfoGroup 获取stream的消费者组相关信息
func TestStreamXInfoGroup(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	groupsInfo, _ := client.XInfoGroups(ctx, streamName).Result()
	for _, group := range groupsInfo {
		fmt.Println("消费者组名称: ", group.Name)
		fmt.Println("消费者数量: ", group.Consumers)
		fmt.Println("已读取但未进行ack的消息数量: ", group.Pending)
	}
}

// TestStreamXInfoConsumers 获取stream的消费者组下的消费者信息
func TestStreamXInfoConsumers(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	// 要真正消费过消息的消费者才会在这里展示
	consumers, _ := client.XInfoConsumers(ctx, streamName, "cg1").Result()
	for _, consumer := range consumers {
		fmt.Println("消费者名称: ", consumer.Name)
		fmt.Println("消费者名称正在进行处理的消息数量: ", consumer.Pending)
		fmt.Println("消费者空闲了多长时间 ms 没有读取消息了: ", consumer.Idle)
	}
}

// TestStreamXReadGroup 使用消费者组读取消息
func TestStreamXReadGroup(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	// 创建消费者组，从头开始消费. start: 要传递起始消息 ID 参数用来初始化last_delivered_id变量
	client.XGroupDestroy(ctx, streamName, "cg1")
	client.XGroupCreate(ctx, streamName, "cg1", "0-0")

	// 消费者1
	consumer1 := &redis.XReadGroupArgs{
		Group:    "cg1",
		Consumer: "consumer-1",
		Streams:  []string{streamName, ">"},
		Count:    1,
		Block:    1,
		NoAck:    false,
	}
	xStreamSlice, _ := client.XReadGroup(ctx, consumer1).Result()
	for _, stream := range xStreamSlice {
		fmt.Println("consumer1 stream: ", stream.Stream)
		for _, message := range stream.Messages {
			fmt.Println("	消费id: ", message.ID)
			fmt.Println("	消费内容: ", message.Values)
		}
	}

	// 消费者2
	consumer2 := &redis.XReadGroupArgs{
		Group:    "cg1",
		Consumer: "consumer-2",
		Streams:  []string{streamName, ">"}, // > 号表示从当前消费组的 last_delivered_id 后面开始读. 每当消费者读取一条消息，last_delivered_id 变量就会前进
		Count:    1,
		Block:    1,
		NoAck:    false,
	}
	xStreamSlice, _ = client.XReadGroup(ctx, consumer2).Result()
	var msgId string
	for _, stream := range xStreamSlice {
		fmt.Println("consumer2 stream: ", stream.Stream)
		for _, message := range stream.Messages {
			fmt.Println("	消费id: ", message.ID)
			fmt.Println("	消费内容: ", message.Values)
			msgId = message.ID
		}
	}

	// 打印消费者组信息
	printInfoGroups(client)

	// 打印消费者组信息
	printInfoConsumers(client)

	client.XAck(ctx, streamName, "cg1", msgId)
	fmt.Println("ack消息：", msgId)

	// 打印消费者组信息
	printInfoGroups(client)

	// 打印消费者组信息
	printInfoConsumers(client)
}

func printInfoGroups(client *redis.Client) {
	groupsInfo, _ := client.XInfoGroups(ctx, streamName).Result()
	for _, group := range groupsInfo {
		fmt.Println("消费者组名称: ", group.Name)
		fmt.Println("消费者数量: ", group.Consumers)
		fmt.Println("已读取但未进行ack的消息数量: ", group.Pending)
	}
}

func printInfoConsumers(client *redis.Client) {
	// 打印消费者组信息
	consumers, _ := client.XInfoConsumers(ctx, streamName, "cg1").Result()
	for _, consumer := range consumers {
		fmt.Println("消费者名称: ", consumer.Name)
		fmt.Println("消费者名称正在进行处理的消息数量: ", consumer.Pending)
		fmt.Println("消费者空闲了多长时间 ms 没有读取消息了: ", consumer.Idle)
	}
}
