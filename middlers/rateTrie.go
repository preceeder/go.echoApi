package middlers

import (
	"golang.org/x/time/rate"
	"strings"
	"sync"
	"time"
)

type RouteLimitConfig struct {
	limit    *rate.Limiter
	lastSeen time.Time
}

// SensitiveTrie
type RateLimiterTrie struct {
	root *TrieNode
}

// NewSensitiveTrie
func NewSensitiveTrie() *RateLimiterTrie {
	return &RateLimiterTrie{
		root: &TrieNode{End: false},
	}
}

func (st *RateLimiterTrie) AddPath(limit *RouteLimitConfig, keys ...string) {
	// 将敏感词转换成utf-8编码后的rune类型(int32)
	trieNode := st.root
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
	if st.root == nil {
		return nil, false
	}
	trieNode := st.root
	newAdd := false
	for _, key := range pathKeys {
		key = strings.Trim(key, "/")
		trieNode, newAdd = trieNode.AddChild(key)
	}
	trieNode.mu.Lock()
	trieNode.Data.limit = rate.NewLimiter(10, 10)
	trieNode.End = true
	trieNode.mu.Unlock()
	return trieNode, newAdd
}

// Match
func (st *RateLimiterTrie) Match(pathKeys ...string) *TrieNode {
	if st.root == nil {
		return nil
	}
	trieNode := st.root
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

	current := st.root
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
		for i := len(lockedNodes) - 1; i >= 0; i-- {
			lockedNodes[i].mu.Unlock()
		}
	}()
	fn(current)

}

func (st *RateLimiterTrie) GC() {
	st.root.gcRecursive()
}

// TrieNode 前缀树节点
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

// 只删除叶子节点
func (tn *TrieNode) gcRecursive() {
	tn.mu.RLock()
	childrenSnapshot := make(map[string]*TrieNode, len(tn.childMap))
	for k, v := range tn.childMap {
		childrenSnapshot[k] = v
	}
	tn.mu.RUnlock()

	for key, child := range childrenSnapshot {
		if !child.End {
			child.gcRecursive()
		}
		child.mu.RLock()
		child.mu.RLock()
		expired := child.End && child.Data != nil && time.Since(child.Data.lastSeen) > time.Minute
		child.mu.RUnlock()
		if expired {
			tn.mu.Lock()
			delete(tn.childMap, key)
			tn.mu.Unlock()
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
