package middlers

import (
	"encoding/json"
	"golang.org/x/time/rate"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type RateLimiterSnapshot struct {
	Path     string    `json:"path"` // "/api/user/login/METHOD/KEY"
	Tokens   float64   `json:"tokens"`
	Rate     float64   `json:"rate"`
	Burst    int       `json:"burst"`
	LastSeen time.Time `json:"last_seen"`
}

type RouteLimitConfig struct {
	limit    *rate.Limiter
	lastSeen time.Time
}

// SensitiveTrie
type RateLimiterTrie struct {
	root atomic.Value // 存 *TrieNode
	//root  *TrieNode
}

// NewSensitiveTrie
func NewSensitiveTrie() *RateLimiterTrie {
	t := &RateLimiterTrie{}
	t.root.Store(&TrieNode{End: false})
	return t
}

func (st *RateLimiterTrie) AddPath(limit *RouteLimitConfig, keys ...string) {
	// 将敏感词转换成utf-8编码后的rune类型(int32)
	trieNode := st.root.Load().(*TrieNode)
	for _, key := range keys {
		key = strings.Trim(key, "/")
		trieNode, _ = trieNode.AddChild(key)
	}
	trieNode.mu.Lock()
	trieNode.End = true
	trieNode.Data = limit
	trieNode.mu.Unlock()
}

func (st *RateLimiterTrie) GetAdd(pathKeys ...string) (*TrieNode, bool) {

	trieNode := st.root.Load().(*TrieNode)
	newAdd := false
	for _, key := range pathKeys {
		key = strings.Trim(key, "/")
		trieNode, newAdd = trieNode.AddChild(key)
	}
	trieNode.mu.Lock()
	trieNode.End = true
	trieNode.mu.Unlock()
	return trieNode, newAdd
}

// Match
func (st *RateLimiterTrie) Match(pathKeys ...string) *TrieNode {
	trieNode := st.root.Load().(*TrieNode)
	for _, key := range pathKeys {
		key = strings.Trim(key, "/")
		trieNode = trieNode.FindChild(key)
		if trieNode == nil {
			return nil
		}
	}
	if !trieNode.End {
		return nil
	}
	return trieNode
}

// 主动删除
func (st *RateLimiterTrie) RemovePath(pathKey string) {
	st.WithLockedPath(pathKey, func(node *TrieNode) {
		if node.End && len(node.childMap) == 0 {
			if node.Parent != nil {
				delete(node.Parent.childMap, node.Key)
			}
			node.End = false
			node.Data = nil
		}
	})
}

// WithLockedPath 会沿着 path 一层层加锁，直到路径末尾节点，执行 fn 后逆序释放锁
func (st *RateLimiterTrie) WithLockedPath(path string, fn func(*TrieNode)) {
	pathSlices := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathSlices) == 0 {
		return
	}

	// 存储沿路径加锁的节点，便于逆序释放
	var lockedNodes []*TrieNode

	current := st.root.Load().(*TrieNode)
	current.mu.Lock()
	lockedNodes = append(lockedNodes, current)

	for _, key := range pathSlices {
		next, ok := current.childMap[key]
		if !ok {
			// 路径不存在，释放已有锁
			for i := len(lockedNodes) - 1; i >= 0; i-- {
				lockedNodes[i].mu.Unlock()
			}
			return
		}
		next.mu.Lock()
		lockedNodes = append(lockedNodes, next)
		current = next
	}

	// 执行用户逻辑（防止 panic 导致锁未释放）
	defer func() {
		if r := recover(); r != nil {
			slog.Error("", "error", r)
		}
		for i := len(lockedNodes) - 1; i >= 0; i-- {
			lockedNodes[i].mu.Unlock()
		}
	}()
	fn(current)

}

func (st *RateLimiterTrie) GC(writeFile bool, filepath string) {
	root := st.root.Load().(*TrieNode)

	// 全新的一棵树
	newRoot := cloneNode(root)
	if writeFile {
		err := st.DumpToFile(newRoot, filepath)
		if err != nil {
			slog.Error("快照写入文件失败", "error", err)
		}
	}

	// 清理副本树上过期节点
	cleanupTrie(newRoot)
	// 替换
	st.root.Store(newRoot)
}

// Snapshot 拷贝快照
func (st *RateLimiterTrie) Snapshot() *TrieNode {
	root := st.root.Load().(*TrieNode)
	return cloneNode(root)
}

func (st *RateLimiterTrie) DumpToFile(triNode *TrieNode, path string) error {
	var snapshots []RateLimiterSnapshot
	//snapshot := st.Snapshot()
	st.collectSnapshot("", triNode, &snapshots)

	data, err := json.Marshal(snapshots)
	if err != nil {
		return err
	}

	// 保证路径存在
	os.MkdirAll(filepath.Dir(path), os.ModePerm)
	return os.WriteFile(path, data, 0644)
}

func (st *RateLimiterTrie) collectSnapshot(prefix string, node *TrieNode, out *[]RateLimiterSnapshot) {
	// 这里是快照拷贝的， 就不需要枷锁了
	if node.End && node.Data != nil {
		*out = append(*out, RateLimiterSnapshot{
			Path:     prefix,
			Rate:     float64(node.Data.limit.Limit()),
			Burst:    node.Data.limit.Burst(),
			Tokens:   node.Data.limit.Tokens(),
			LastSeen: node.Data.lastSeen,
		})
	}

	for k, child := range node.childMap {
		newPrefix := prefix + "/" + k
		st.collectSnapshot(newPrefix, child, out)
	}
}

// TrieNode 前缀树节点,   path, method, key
type TrieNode struct {
	mu       sync.RWMutex
	childMap map[string]*TrieNode // 本节点下的所有子节点
	Data     *RouteLimitConfig    // 在最后一个节点保存完整的一个内容
	End      bool                 // 标识是否最后一个节点
	Parent   *TrieNode            // 指向父节点的指针
	Key      string
}

// AddChild 前缀树添加字节点
// bool 是否新增
func (tn *TrieNode) AddChild(c string) (*TrieNode, bool) {
	tn.mu.Lock()
	defer tn.mu.Unlock()
	if tn.childMap == nil {
		tn.childMap = make(map[string]*TrieNode)
	}

	if trieNode, ok := tn.childMap[c]; ok {
		return trieNode, false
	} else {
		// 不存在
		data := &TrieNode{
			mu:       sync.RWMutex{},
			childMap: nil,
			End:      false,
			Data:     &RouteLimitConfig{},
			Parent:   tn,
			Key:      c,
		}
		tn.childMap[c] = data
		return data, true
	}
}

func (tn *TrieNode) RemoveChild(key string) {
	tn.mu.Lock()
	defer tn.mu.Unlock()
	// 是叶子结点
	child, ok := tn.childMap[key]
	if ok {
		child.mu.RLock()
		isLeaf := child.End && len(child.childMap) == 0
		child.mu.RUnlock()

		if isLeaf {
			delete(tn.childMap, key)
		}
	}
}

// FindChild 前缀树查找字节点
func (tn *TrieNode) FindChild(c string) *TrieNode {
	tn.mu.RLock()
	defer tn.mu.RUnlock()
	if tn.childMap == nil {
		return nil
	}
	return tn.childMap[c]
}

// 节点拷贝
func cloneNode(node *TrieNode) *TrieNode {
	if node == nil {
		return nil
	}
	newNode := &TrieNode{
		mu:       sync.RWMutex{},
		childMap: make(map[string]*TrieNode),
		End:      node.End,
		Data:     node.Data, // 浅拷贝，RateLimiter是线程安全的
		Key:      node.Key,
	}
	node.mu.RLock()
	for k, v := range node.childMap {
		child := cloneNode(v)
		child.Parent = newNode
		newNode.childMap[k] = child
	}
	node.mu.RUnlock()
	return newNode
}

func cleanupTrie(node *TrieNode) {

	for key, child := range node.childMap {
		if child.End && child.Data != nil && time.Since(child.Data.lastSeen) > time.Minute {
			delete(node.childMap, key)
		} else {
			cleanupTrie(child)
		}
	}
}
