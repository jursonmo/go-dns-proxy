package dnsproxy

import (
	"fmt"
	"sync"
)

const MaxLen = 255
const InitCap = 28 * 2 //为啥是28*2 ? 26个字母加.* 就28个，再加上数字10个，"-"
//国际域名可以使用的字符是：英文26个字母和10个阿拉伯数字以及横杠"－"（减号

type Trie struct {
	sync.RWMutex
	IsComplete bool
	Next       map[rune]*Trie
	Record     interface{}
}

func NewTrie() *Trie {
	trie := &Trie{}
	trie.IsComplete = false
	trie.Next = make(map[rune]*Trie, InitCap)
	return trie
}

func (root *Trie) Insert(name string, data interface{}) {
	word := reverseString(name)
	location := root

	for _, c := range word {
		if location.Next[c] == nil {
			newNode := new(Trie)
			newNode.IsComplete = false
			newNode.Next = make(map[rune]*Trie, InitCap)
			location.Next[c] = newNode
		}
		temp := location.Next[c]
		location = temp
	}

	location.IsComplete = true
	location.Record = data
}

func (root *Trie) Delete(name string) {
	word := reverseString(name)
	location := root

	notFound := false
	for _, c := range word {
		if location.Next[c] == nil {
			notFound = true
			break
		}
		temp := location.Next[c]
		location = temp
	}

	if !notFound && location.IsComplete {
		location.Record = nil
	}
}

//翻转域名，逐个对比，同时用记录preSplitLocation "."的节点，因为这个可能是 ".*",
//在没有找到最长匹配的情况下，如果preSplitLocation 是 ".*",可以认为模糊匹配成功
func (root *Trie) Find(domainName string) (interface{}, error) {
	word := reverseString(domainName)
	location := root

	var preSplitLocation *Trie

	for _, c := range word {
		if location.Next[c] == nil {
			location = nil
			break
		}

		temp := location.Next[c]
		location = temp

		if string(c) == "." {
			preSplitLocation = location
		}
	}

	if location != nil { //如果查找key 相对短了，trie 保存的key更长 ,location 就不为空
		if location.IsComplete {
			return location.Record, nil
		} else {
			preSplitLocation = location.Next['.']
		}
	}

	if preSplitLocation != nil {
		if newLocation := preSplitLocation.Next['*']; newLocation != nil {
			if newLocation.IsComplete {
				return newLocation.Record, nil
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func reverseString(s string) string {
	r := []rune(s)
	for l, h := 0, len(r)-1; l < h; l, h = l+1, h-1 {
		r[l], r[h] = r[h], r[l]
	}

	return string(r)
}
