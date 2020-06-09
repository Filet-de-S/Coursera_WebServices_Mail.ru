package main

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"
)

type channels struct {
	in  chan interface{}
	out chan interface{}
}

func ExecutePipeline(freeFlowJobs ...job) {
	wg := &sync.WaitGroup{}
	chnl := channels{in: make(chan interface{}, 1)}

	for n := range freeFlowJobs {
		chnl.out = make(chan interface{}, 1)
		wg.Add(1)
		go func(function job, chnl channels, wg *sync.WaitGroup) {
			function(chnl.in, chnl.out)
			close(chnl.out)
			wg.Done()
		}(freeFlowJobs[n], chnl, wg)
		chnl.in = chnl.out
	}
	wg.Wait()
}

func SingleHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	start := time.Now()

	for val := range in {
		time.Sleep(11 * time.Millisecond)
		transit := val.(int)
		wg.Add(1)
		go func(value int, wg *sync.WaitGroup) {
			fmt.Println(value, "SingleHash data", value)

			defer wg.Done()
			valInt := strconv.Itoa(value)
			var hashTotal string
			hash0 := make(chan string)
			hash1 := make(chan string)
			hash2 := make(chan string)

			go func() {
				hash0 <- DataSignerMd5(valInt)
			}()
			go func() {
				hash1 <- DataSignerCrc32(valInt)
			}()
			go func() {
				hash2 <- DataSignerCrc32(<-hash0)
			}()
			hashTotal = <-hash1 + "~" + <-hash2
			fmt.Println(value, "SingleHash result", hashTotal)
			out <- hashTotal
		}(transit, wg)
	}
	wg.Wait()
	end := time.Since(start)
	fmt.Println("SingleHash dur:", end)

}

func MultiHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	start := time.Now()

	for val := range in {
		transit := val.(string)
		fmt.Println("MultiHash started", transit)
		wg.Add(1)
		go func(value string, wg *sync.WaitGroup) {
			defer wg.Done()
			multi := make(chan string, 6)
			for th := 0; th < 6; th++ {
				go func(th int, value string) {
					thstr := strconv.Itoa(th)
					multi <- thstr + DataSignerCrc32(thstr+value)
				}(th, value)
			}

			result := make([]string, 6)
			counter := 0
			for v := range multi {
				n, _ := strconv.ParseInt(v[:1], 10, 10)
				result[n] = v[1:]
				counter++
				if counter == 6 {
					close(multi)
				}
			}

			var forOut string
			for n := range result {
				forOut += result[n]
			}
			out <- forOut
			fmt.Println("MultiHash", value, "result:\n", forOut)
		}(transit, wg)
	}
	wg.Wait()
	end := time.Since(start)
	fmt.Println("MultiHash time used:", end)
}

func CombineResults(in, out chan interface{}) {
	start := time.Now()

	combined := make([]string, 0)
	i := 0
	for val := range in {
		combined = append(combined, val.(string))
		fmt.Println("Combined: ", strconv.Itoa(i), combined)
	}
	fmt.Println("Comb TIME:", time.Since(start))

	sort.SliceStable(combined, func(i, j int) bool {
		return combined[i] < combined[j]
	})

	var result string
	for n, _ := range combined {
		if n == 0 {
			result = combined[n]
		} else {
			result += "_" + combined[n]
		}
	}
	out <- result
}

//func main() {
//	slicedLine := []int{0, 1, 1, 2, 3, 5, 8}
//
//	freeFlowJobs := []job{
//		job(func(in, out chan interface{}) {
//			for _, val := range slicedLine {
//				out <- val
//			}
//		}),
//		job(SingleHash),
//		job(MultiHash),
//		job(CombineResults),
//		job(func(in, out chan interface{}) {
//			dataRaw := <-in
//			data, ok := dataRaw.(string)
//			if !ok {
//				fmt.Errorf("cant convert result data to string")
//			}
//			fmt.Println("YRA! \n", data)
//		}),
//	}
//
//	start := time.Now()
//	ExecutePipeline(freeFlowJobs...)
//	fmt.Println("time: ", time.Since(start))
//}
