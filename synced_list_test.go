package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestSyncedListAdd(t *testing.T) {
	n, cl := 10, &syncedlist{}
	expectedArr := make([]string, n)
	wg := new(sync.WaitGroup)
	wg.Add(n)

	for i := range n {
		expectedArr[i] = strconv.Itoa(i)
		go func(i int) {
			defer wg.Done()

			cl.add(strconv.Itoa(i))
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
