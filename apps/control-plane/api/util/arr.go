package util

func SplitArr[T any](arr []T, num int64) [][]T {
	max := int64(len(arr))
	//判断数组大小是否小于等于指定分割大小的值，是则把原数组放入二维数组返回
	if max <= num {
		return [][]T{arr}
	}
	//获取应该数组分割为多少份
	var quantity int64
	if max%num == 0 {
		quantity = max / num
	} else {
		quantity = (max / num) + 1
	}
	//声明分割好的二维数组
	var segments = make([][]T, 0)
	//声明分割数组的截止下标
	var start, end, i int64
	for i = 1; i <= quantity; i++ {
		end = i * num
		if i != quantity {
			segments = append(segments, arr[start:end])
		} else {
			segments = append(segments, arr[start:])
		}
		start = i * num
	}
	return segments
}

// ArraysEqualPrefix 以a为主
func ArraysEqualPrefix(a, b []string) bool {
	// 如果两个数组长度不相等，直接返回false
	if len(a) > len(b) {
		return false
	}
	// 遍历两个数组的元素，逐一比较它们的值
	for i, item := range a {
		if item != b[i] {
			return false
		}
	}
	return true
}

func ArrContainKeys(arr []string, keys []string) bool {
	for _, item := range keys {
		if !ArrContain(arr, item) {
			return false
		}
	}
	return true
}

func ArrContain(arr []string, key string) bool {
	for _, item := range arr {
		if item == key {
			return true
		}
	}
	return false
}
