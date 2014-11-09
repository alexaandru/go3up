package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestSyncedListAdd(t *testing.T) {
	n, cl := 10, &syncedlist{}
	expectedArr := make([]string, n)
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for i := 0; i < n; i++ {
		expectedArr[i] = fmt.Sprintf("%d", i)
		go func(i int) {
			defer wg.Done()
			cl.add(fmt.Sprintf("%d", i))
		}(i)
	}
	wg.Wait()
	expected := strings.Join(expectedArr, ":")
	sort.Strings(cl.list)
	actual := strings.Join(cl.list, ":")

	if expected != actual {
		t.Errorf("Expected %s\n got %s\n", expected, actual)
	}
}
