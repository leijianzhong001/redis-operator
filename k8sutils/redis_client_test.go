package k8sutils

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"reflect"
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
