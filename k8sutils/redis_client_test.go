package k8sutils

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/lucasepe/codename"
	"reflect"
	"strings"
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

func TestCodename(t *testing.T) {
	rng, _ := codename.DefaultRNG()
	for i := 0; i < 100; i++ {
		str := codename.Generate(rng, 50)
		fmt.Println(len(str), ": ", str)
	}
	fmt.Println(len("忍者"))
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

// 总内存消耗 = (`32` + `16` + `key_SDS`大小＋`val_SDS`大小) * key个数＋bucket个数 * 8
func TestMemoryEvaluationString(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	var all int64 = 0
	var keyCount int64 = 0

	rng, _ := codename.DefaultRNG()
	kV := make(map[string]string, 1000)
	for i := 1; i <= 3000000; i++ {
		key := "snrs:" + fmt.Sprintf("%d", i)
		value := codename.Generate(rng, 50)
		kV[key] = value
		if len(kV) >= 1000 {
			pipeline := client.Pipeline()
			for k, v := range kV {
				pipeline.Set(ctx, k, v, redis.KeepTTL)
			}
			exec, err := pipeline.Exec(ctx)
			if err != nil {
				panic(err)
			}
			fmt.Printf("批次 %d 执行结果:%v\n", i/1000, exec)
			// 清空k_v,这种方式会被编译器优化，事实上比make(map[string]string, 1000)更快
			for k := range kV {
				delete(kV, k)
			}
		}
		keyCount++
		// 4是这个长度下sds结构体的占用
		keySize := int64(32 + 16 + len(key) + 4 + len(value) + 4)
		all += keySize
	}

	// 将kV中剩余的key也存进去
	pipeline := client.Pipeline()
	for k, v := range kV {
		pipeline.Set(ctx, k, v, redis.KeepTTL)
	}
	pipeline.Exec(ctx)

	// 300w的key, 其对应的bucket的数量是2^22=4194304
	bucketCount := int64(4194304)
	// 加上bucket的长度
	all += bucketCount * 8
	fmt.Println("Done...")
	fmt.Printf("key数量：%d, 数据总大小(B)：%d, 数据总大小(MB)：%d", keyCount, all, all/1024/1024)

	// 总内存消耗 = (`32` + `16` + `key_SDS`大小＋`val_SDS`大小) * key个数＋bucket个数 * 8
	// len(key)=12 len(value)=70
	// (48 + 16 + 74) * 3000000 + 4194304 * 8 = 414000000 + 33554432 = 447554432 = 426MB
}

// 单个key内存开销(jemalloc) = 32 + key_SDS大小 + 16 + 96 + (32 + field_SDS大小 + val_SDS大小) * field个数 + field_bucket个数 * 8
func TestMemoryEvaluationSingleHash(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	var allDataSize int64 = 32 + 6 + 4 + 16 + 96
	var fieldCount int64 = 0

	rng, _ := codename.DefaultRNG()
	fieldAndValue := make(map[string]string, 1000)
	key := "goland"
	keyCount := 1
	for i := 1; i <= 30000000; i++ {
		field := "snrs:" + fmt.Sprintf("%d", i)
		value := codename.Generate(rng, 50)
		fieldAndValue[field] = value
		if len(fieldAndValue) >= 10000 {
			result, err := client.HSet(ctx, key, fieldAndValue).Result()
			if err != nil {
				panic(err)
			}
			fmt.Printf("批次 %d 执行结果:%v\n", i/10000, result)
			// 清空k_v,这种方式会被编译器优化，事实上比make(map[string]string, 1000)更快
			for f := range fieldAndValue {
				delete(fieldAndValue, f)
			}
		}
		fieldCount++
		// 4是这个长度下sds结构体的占用
		fieldSize := int64(32 + len(field) + 4 + len(value) + 4)
		allDataSize += fieldSize
	}
	// 将kV中剩余的key也存进去
	client.HSet(ctx, key, fieldAndValue)

	// 3000w的key, 其对应的bucket的数量是2^25=33554432
	fieldBucketCount := int64(33554432)
	// 加上bucket的长度
	allDataSize += fieldBucketCount * 8
	fmt.Println("Done...")
	fmt.Printf("key长度：%d\n", len(key))
	fmt.Printf("field平均长度：%d, value平均长度：%d\n", 12, 70)
	fmt.Printf("key数量：%d, field数量：%d, 数据总大小(B)：%d, 数据总大小(MB)：%d\n", keyCount, fieldCount, allDataSize, allDataSize/1024/1024)
}

//总内存开销 = [32 + key_SDS大小 + 16 + 96 + (32 + field_SDS大小 + val_SDS大小) * field个数 + field_bucket个数 * 8] * key个数 + key_bucket个数 * 8
// 精简公式一下为：
//总内存开销 = [key_SDS大小 + 144 + (32 + field_SDS大小 + val_SDS大小) * field个数 + field_bucket个数 * 8] * key个数 + key_bucket个数 * 8
func TestMemoryEvaluationMultiHash(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	var allKeySize int64 = 0
	keyCount := 1000
	keyBucketCount := 1024
	fieldCount := 30000
	// 2^15 = 32768
	fieldBucketCount := int64(32768)
	rng, _ := codename.DefaultRNG()
	fieldAndValue := make(map[string]string, 1000)
	for i := 0; i < keyCount; i++ {
		key := "goland-key-" + fmt.Sprintf("%04d", i)
		// 当前key对应的所有field和value的大小
		allFieldSize := int64(0)
		for j := 0; j < fieldCount; j++ {
			field := "goland-field-" + fmt.Sprintf("%05d", j)
			value := codename.Generate(rng, 50)
			fieldAndValue[field] = value

			// 当前key-value的大小
			fieldSize := int64(32 + len(field) + 4 + len(value) + 4)
			allFieldSize += fieldSize
		}
		result, err := client.HSet(ctx, key, fieldAndValue).Result()
		if err != nil {
			panic(err)
		}
		fmt.Printf("key %s 执行结果:%v\n", key, result)
		for f := range fieldAndValue {
			// 清空k_v,这种方式会被编译器优化，事实上比make(map[string]string, 1000)更快
			delete(fieldAndValue, f)
		}

		// 当前key大小
		//单个key大小 = dictEntry大小 + key_SDS大小 + redisObject大小 + dict大小 + (dictEntry大小 + field_SDS大小 + val_SDS大小) * field个数 + field_bucket个数 * 指针大小
		//单个key大小 = 32 + key_SDS大小 + 16 + 96 + (32 + field_SDS大小 + val_SDS大小) * field个数 + field_bucket个数 * 8
		keySize := int64(32+len(key)+4+16+96) + allFieldSize + fieldBucketCount*8
		allKeySize += keySize
	}

	// 另外再加上 key_bucket 占用
	allKeySize += int64(keyBucketCount * 8)

	fmt.Println("Done...")
	fmt.Printf("key数量：%d, key长度：%d\n", keyCount, 15)
	fmt.Printf("单key中field数量：%d, field长度：%d, value平均长度：%d\n", fieldCount, 18, 70)
	fmt.Printf("数据总大小(B)：%d,  数据总大小(MB)：%d\n", allKeySize, allKeySize/1024/1024)
}

// List
// 单个key的内存消耗 = 32 + key_SDS大小 + 16 + 48 + quicklist.len * 32 + 元素个数 * 8192
// 总内存消耗计算方式为：
// 总内存消耗 = (32 + key_SDS大小 + 16 + 48 + quicklist.len * 32 + quicklist.len * 8192) * key个数＋bucket个数 * 8
func TestMemoryEvaluationSingleList(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	rng, _ := codename.DefaultRNG()
	elements := make([]string, 0, 10000)
	key := "goland"
	elementCount := 30000000

	// ========================================================> ziplist相关常量定义 <========================================================
	// 上一个元素长度的编码长度，因为当前元素的长度都在70左右，小于254，所以这里就是1个字节
	previousEntryLength := 1
	// 当前元素编码部分长度。因为元素长度再70左右，大于63且小于16383，所以是2个字节
	encoding := 2
	zipListHeaderSize := 10
	zipListEndSize := 1
	maxZipListSize := 8192
	// ========================================================> ziplist相关常量定义 <========================================================

	// 当前ziplist长度
	currentZipListSize := zipListHeaderSize + zipListEndSize
	quickListNodeCount := 0
	for i := 1; i <= elementCount; i++ {
		value := codename.Generate(rng, 50)
		elements = append(elements, value)
		if len(elements) >= 10000 {
			result, err := client.LPush(ctx, key, elements).Result()
			if err != nil {
				panic(err)
			}
			fmt.Printf("批次 %d 执行结果:%v\n", i/10000, result)
			// 清空切片
			elements = elements[0:0]
		}

		// ziplist结构 = ziplist header + ziplist end + 所有entry
		// entry结构 = previous_entry_length + encoding + len(content) = 1 + 2 + len(content) = 3 + len(content)

		// 当前元素所在的entry的长度
		currentEntrySize := previousEntryLength + encoding + len(value)
		if currentZipListSize+currentEntrySize > maxZipListSize {
			// 说明当前ziplist已经满了，此时将quickListNode数量 + 1
			quickListNodeCount++
			// 满了以后重置当前zipList长度，开始下一个zipList累加
			currentZipListSize = zipListHeaderSize + zipListEndSize
		} else {
			// 当前ziplist没满，继续累加
			currentZipListSize += currentEntrySize
		}
	}

	// 肯定最后的元素凑不满一个quickListNode， 所以这里还需要+1
	quickListNodeCount++

	// 单个key的内存消耗 = 32 + key_SDS大小 + 16 + 48 + quicklist.len * 32 + quicklist.len * 8192
	allDataSize := int64(32 + len(key) + 4 + 16 + 48 + quickListNodeCount*32 + quickListNodeCount*8192)

	fmt.Println("Done...")
	fmt.Printf("数据总大小(B)：%d, 数据总大小(MB)：%d\n", allDataSize, allDataSize/1024/1024)
	fmt.Printf("key长度：%d\n", len(key))
	fmt.Printf("element平均长度: %d, element数量: %d\n", 70, elementCount)
	// 2001.3577880859375
}

// List
// 单个key的内存消耗 = 32 + key_SDS大小 + 16 + 48 + quicklist.len * 32 + quicklist.len * 8192
// 总内存消耗计算方式为：
// 总内存消耗 = (32 + key_SDS大小 + 16 + 48 + quicklist.len * 32 + quicklist.len * 8192) * key个数＋bucket个数 * 8
func TestMemoryEvaluationMultiList(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	rng, _ := codename.DefaultRNG()
	elements := make([]string, 0, 30000)
	elementCount := 30000
	keyCount := 1000
	allListOverhead := int64(0)

	// ========================================================> ziplist相关常量定义 <========================================================
	// 上一个元素长度的编码长度，因为当前元素的长度都在70左右，小于254，所以这里就是1个字节
	previousEntryLength := 1
	// 当前元素编码部分长度。因为元素长度再70左右，大于63且小于16383，所以是2个字节
	encoding := 2
	zipListHeaderSize := 10
	zipListEndSize := 1
	maxZipListSize := 8192
	// ========================================================> ziplist相关常量定义 <========================================================

	for i := 1; i <= keyCount; i++ {
		// 当前ziplist长度
		currentZipListSize := zipListHeaderSize + zipListEndSize
		// 当前list的quickListNode节点数量
		currentListNodeCount := 0
		key := "goland-key-" + fmt.Sprintf("%04d", i)
		for j := 0; j < elementCount; j++ {
			value := codename.Generate(rng, 50)
			elements = append(elements, value)

			// 当前元素所在的entry的长度
			currentEntrySize := previousEntryLength + encoding + len(value)
			if currentZipListSize+currentEntrySize > maxZipListSize {
				// 说明当前ziplist已经满了，此时将quickListNode数量 + 1
				currentListNodeCount++
				// 满了以后重置当前zipList长度，开始下一个zipList累加
				currentZipListSize = zipListHeaderSize + zipListEndSize
			} else {
				// 当前ziplist没满，继续累加
				currentZipListSize += currentEntrySize
			}
		}

		// 插入到redis中
		result, err := client.LPush(ctx, key, elements).Result()
		if err != nil {
			panic(err)
		}
		fmt.Printf("批次 %d 执行结果:%v\n", i, result)
		// 清空切片
		elements = elements[0:0]

		// ziplist结构 = ziplist header + ziplist end + 所有entry
		// entry结构 = previous_entry_length + encoding + len(content) = 1 + 2 + len(content) = 3 + len(content)

		// 肯定最后的元素凑不满一个quickListNode， 所以这里还需要+1
		currentListNodeCount++
		// 单个key的内存消耗 = 32 + key_SDS大小 + 16 + 48 + quicklist.len * 32 + quicklist.len * 8192
		currentListOverhead := int64(32 + len(key) + 4 + 16 + 48 + currentListNodeCount*32 + currentListNodeCount*8192)

		// 总内存消耗 = (32 + key_SDS大小 + 16 + 48 + quicklist.len * 32 + quicklist.len * 8192) * key个数＋bucket个数 * 8
		allListOverhead += currentListOverhead
	}

	keyBucketCount := 1024
	allListOverhead += int64(keyBucketCount * 8)

	fmt.Println("Done...")
	fmt.Printf("key长度：%d, key数量: %d\n", 15, keyCount)
	fmt.Printf("元素平均长度: %d, 每个list的元素数量: %d\n", 70, elementCount)
	fmt.Printf("数据总大小(B)：%d, 数据总大小(MB)：%d\n", allListOverhead, allListOverhead/1024/1024)
}

// Set
//单个key内存开销 = 24 + key_SDS大小 + 16 + 96 + (24 + val_SDS大小) * value个数 + value_bucket个数 * 8
// 按照jemalloc分配规则分配之后的内存为：
//单个key内存开销(jemalloc) = 32 + key_SDS大小 + 16 + 96 + (32 + val_SDS大小) * value个数 + value_bucket个数 * 8

// 总内存消耗 = [dictEntry大小 + key_SDS大小 + redisObject大小 + dict大小 + (dictEntry大小 + val_SDS大小) * value个数 + value_bucket个数 * 指针大小] * key个数 + key_bucket个数 * 指针大小
//总内存开销 = [32 + key_SDS大小 + 16 + 96 + (32 + val_SDS大小) * value个数 + value_bucket个数 * 8] * key个数 + key_bucket个数 * 8
// 精简公式一下为：
//总内存开销 = [key_SDS大小 + 144 + (32 + val_SDS大小) * value个数 + value_bucket个数 * 8] * key个数 + key_bucket个数 * 8
func TestMemoryEvaluationSingleSet(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	rng, _ := codename.DefaultRNG()
	elements := make([]string, 0, 10000)
	key := "goland"
	elementCount := 30000000
	allOverhead := int64(32 + len(key) + 4 + 16 + 96)

	for i := 1; i <= elementCount; i++ {
		value := codename.Generate(rng, 50)
		elements = append(elements, value)
		if len(elements) >= 10000 {
			result, err := client.SAdd(ctx, key, elements).Result()
			if err != nil {
				panic(err)
			}
			fmt.Printf("批次 %d 执行结果:%v\n", i/10000, result)
			// 清空切片
			elements = elements[0:0]
		}
		// dictEntry + value本身 + sds结构体
		valueOverhead := int64(32 + len(value) + 4)
		allOverhead += valueOverhead
	}
	// 肯定最后的元素凑不满一个quickListNode， 所以这里还需要再执行一次
	client.SAdd(ctx, key, elements)

	// 3000w的value, 其对应的bucket的数量是2^25=33554432
	valueBucketCount := int64(33554432)
	allOverhead += valueBucketCount * 8

	// 3227.61519622802734375
	fmt.Println("Done...")
	fmt.Printf("key长度：%d\n", len(key))
	fmt.Printf("value平均长度: %d, value数量: %d\n", 70, elementCount)
	fmt.Printf("数据总大小(B)：%d, 数据总大小(MB)：%d\n", allOverhead, allOverhead/1024/1024)
}

// Set
//单个key内存开销 = 24 + key_SDS大小 + 16 + 96 + (24 + val_SDS大小) * value个数 + value_bucket个数 * 8
// 按照jemalloc分配规则分配之后的内存为：
//单个key内存开销(jemalloc) = 32 + key_SDS大小 + 16 + 96 + (32 + val_SDS大小) * value个数 + value_bucket个数 * 8

// 总内存消耗 = [dictEntry大小 + key_SDS大小 + redisObject大小 + dict大小 + (dictEntry大小 + val_SDS大小) * value个数 + value_bucket个数 * 指针大小] * key个数 + key_bucket个数 * 指针大小
//总内存开销 = [32 + key_SDS大小 + 16 + 96 + (32 + val_SDS大小) * value个数 + value_bucket个数 * 8] * key个数 + key_bucket个数 * 8
// 精简公式一下为：
//总内存开销 = [key_SDS大小 + 144 + (32 + val_SDS大小) * value个数 + value_bucket个数 * 8] * key个数 + key_bucket个数 * 8
func TestMemoryEvaluationMultiSet(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	rng, _ := codename.DefaultRNG()
	elements := make([]string, 0, 30000)
	elementCount := 30000
	keyCount := 1000

	//总内存开销 = [32 + key_SDS大小 + 16 + 96 + (32 + val_SDS大小) * value个数 + value_bucket个数 * 8] * key个数 + key_bucket个数 * 8
	allSetOverhead := int64(0)

	// 单个key内存开销(jemalloc) = 32 + key_SDS大小 + 16 + 96 + (32 + val_SDS大小) * value个数 + value_bucket个数 * 8
	for i := 1; i <= keyCount; i++ {
		key := "goland-key-" + fmt.Sprintf("%04d", i)
		currentKeyOverhead := int64(32 + len(key) + 16 + 96)
		for j := 0; j < elementCount; j++ {
			value := codename.Generate(rng, 50)
			elements = append(elements, value)
			currentKeyOverhead += int64(32 + len(value) + 4)
		}
		// 插入到redis中
		result, err := client.SAdd(ctx, key, elements).Result()
		if err != nil {
			panic(err)
		}
		fmt.Printf("批次 %d 执行结果:%v\n", i, result)

		// 加上value_bucket占用 2^15 = 32768
		valueBucket := int64(32768)
		currentKeyOverhead += valueBucket * 8

		// 清空切片
		elements = elements[0:0]

		// 累加到总值中
		allSetOverhead += currentKeyOverhead
	}

	// 加上key bucket占用
	keyBucketCount := 1024
	allSetOverhead += int64(keyBucketCount * 8)

	// 3221.77758026123046875
	fmt.Println("Done...")
	fmt.Printf("key长度：%d, key数量: %d\n", 15, keyCount)
	fmt.Printf("元素平均长度: %d, 每个list的元素数量: %d\n", 70, elementCount)
	fmt.Printf("数据总大小(B)：%d, 数据总大小(MB)：%d\n", allSetOverhead, allSetOverhead/1024/1024)
}

func TestMemoryEvaluationSingleSortedset(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	rng, _ := codename.DefaultRNG()
	elements := make([]*redis.Z, 0, 10000)
	key := "goland"
	elementCount := 30000000
	allOverhead := int64(32 + len(key) + 4 + 16 + 16 + 96 + 32) // dictEntry + key_sds + value_redisObject + zset + dict + skiplist

	for i := 1; i <= elementCount; i++ {
		value := codename.Generate(rng, 50)
		elements = append(elements, &redis.Z{Score: float64(i), Member: value})
		if len(elements) >= 10000 {
			result, err := client.ZAdd(ctx, key, elements...).Result()
			if err != nil {
				panic(err)
			}
			fmt.Printf("批次 %d 执行结果:%v\n", i/10000, result)
			// 清空切片
			elements = elements[0:0]
		}
		// 单key内存开销 = dictEntry + key_sds + value_redisObject + zset + dict + dictEntry * n + 8 * elementbucketCount($elementbucketCount= 2^b, elementbucketCount >= n$) + skiplist + zskiplistNode * n
		// 单key内存开销 = 32 + 4~18 + 16 + 16 + 96 + 32 * n + (4~18} * n + 8 * n + 8 * elementbucketCount($elementbucketCount= 2^b, elementbucketCount>= n$) + 32 + (4~18 + len(context) + 16 + 16 * levelSize) * n
		currentElementOverhead := 32 + len(value) + 4 + 16
		allOverhead += int64(currentElementOverhead)
	}
	// 肯定最后的元素凑不满一个quickListNode， 所以这里还需要再执行一次
	client.ZAdd(ctx, key, elements...)

	// 3000w的element, 其对应的bucket的数量是2^25=33554432
	elementBucketCount := int64(33554432)
	// zset内部的dict同样也需要rehash，所以要加上ht[1]中rehash的占用
	allOverhead += elementBucketCount * 8

	// 计算3000w key的情况下，索引层数以及每层元素数量分布情况
	levelAndElementCount := make([]int64, 0, 32)
	currentLevelElementCount := int64(elementCount)
	for true {
		if currentLevelElementCount >= 1 {
			levelAndElementCount = append(levelAndElementCount, currentLevelElementCount)
		} else {
			break
		}
		currentLevelElementCount = currentLevelElementCount / 4
	}
	// levelSizeEnum := []int64{1, 7, 28, 114, 457, 1831, 7324, 29296, 117188, 468750, 1875000, 7500000, 30000000}
	fmt.Println("levelAndElementCount: ", levelAndElementCount)

	// 总容量加上多层索引的开销
	fmt.Println("allOverhead before: ", allOverhead/1024/1024)
	for _, count := range levelAndElementCount {
		allOverhead += count * 16
	}
	fmt.Println("allOverhead after: ", allOverhead/1024/1024)

	// 4538.916732788086
	fmt.Println("Done...")
	fmt.Printf("key长度：%d\n", len(key))
	fmt.Printf("value平均长度: %d, value数量: %d\n", 70, elementCount)
	fmt.Printf("数据总大小(B)：%d, 数据总大小(MB)：%d\n", allOverhead, allOverhead/1024/1024)
}

func TestMemoryEvaluationMultiSortedset(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	rng, _ := codename.DefaultRNG()
	elements := make([]*redis.Z, 0, 30000)
	keyCount := 1000
	elementCount := 30000
	allOverhead := int64(0) // dictEntry + key_sds + value_redisObject + zset + dict + skiplist

	for i := 1; i <= keyCount; i++ {
		key := "goland-key-" + fmt.Sprintf("%04d", i)
		// 单key内存开销 = dictEntry + key_sds + value_redisObject + zset + dict + dictEntry * n + 8 * elementbucketCount($elementbucketCount= 2^b, elementbucketCount >= n$) + skiplist + zskiplistNode * n
		currentKeyOverhead := int64(32 + len(key) + 4 + 16 + 16 + 96 + 32) // 32是 zskiplist 结构体长度
		for j := 1; j <= elementCount; j++ {
			value := codename.Generate(rng, 50)
			elements = append(elements, &redis.Z{Score: float64(j), Member: value})

			currentKeyOverhead += int64(32 + len(value) + 4 + 16) // 这里的len(value) + 4是zkiplistnode中ele的长度，16是zkiplistnode中score和*backward的长度
		}

		result, err := client.ZAdd(ctx, key, elements...).Result()
		if err != nil {
			panic(err)
		}
		fmt.Printf("批次 %d 执行结果:%v\n", i, result)
		// 清空切片
		elements = elements[0:0]

		// 每个key的开销要加上 elementbucket长度 2^15
		elementBucketCount := int64(32768)
		currentKeyOverhead += elementBucketCount * 8

		// 计算30000 key的情况下，索引层数以及每层元素数量分布情况,其中切片下标为索引层数，下标对应的值为该层元素数量
		levelAndElementCount := make([]int64, 0, 32)
		// 第一层直接就是元素个数，每个元素至少有一层索引
		currentLevelElementCount := int64(elementCount)
		for {
			if currentLevelElementCount >= 1 {
				levelAndElementCount = append(levelAndElementCount, currentLevelElementCount)
			} else {
				break
			}
			currentLevelElementCount = currentLevelElementCount / 4
		}
		// levelSizeEnum := []int64{1, 7, 29, 117, 468, 1875, 7500, 30000}
		fmt.Println("levelAndElementCount: ", levelAndElementCount)

		// 当前key容量要加上多层索引的开销
		for _, count := range levelAndElementCount {
			currentKeyOverhead += count * 16
		}

		// 累加到总值中
		allOverhead += currentKeyOverhead
	}

	// 3000w的element, 其对应的bucket的数量是2^25=33554432
	keyBucketCount := int64(1024)
	// zset内部的dict同样也需要rehash，所以要加上ht[1]中rehash的占用
	allOverhead += keyBucketCount * 8

	// 4533.5869140625
	fmt.Println("Done...")
	fmt.Printf("key长度：%d\n", 15)
	fmt.Printf("value平均长度: %d, value数量: %d\n", 70, elementCount)
	fmt.Printf("数据总大小(B)：%d, 数据总大小(MB)：%d\n", allOverhead, allOverhead/1024/1024)
}

func TestXXX(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:     "10.243.66.12:6380",
		Password: "",
		DB:       0,
	})

	infoResult, _ := client.Info(ctx, "Persistence").Result()
	fmt.Println(infoResult)
	fmt.Println()
	for _, ele := range strings.Split(infoResult, "\r\n") {
		nameAndValue := strings.Split(ele, ":")
		if len(nameAndValue) < 2 {
			continue
		}
		name := nameAndValue[0]
		value := nameAndValue[1]
		fmt.Printf("configName: %s, value: %s\n", name, value)
	}
}
