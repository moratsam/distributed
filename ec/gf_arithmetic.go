package main 
import (
	"fmt"

)

var verbose = true
var generator = 0x11d

func printAsBinary(name string, b byte) {
	fmt.Printf("%v: ", name)
	for i:=0; i<8; i++ {
		bit := b >> (7-i) & 1
		fmt.Printf("%c", '0'+bit)
	}
	fmt.Println()
}

func add(a, b byte) byte {
	result := a^b;
	if verbose {
		printAsBinary("a", a)
		printAsBinary("b", b)
		printAsBinary("a+b", result)
	}
	return result
}

// calculate bit length
func length(a int) int {
	result := 0
	for i:=0; a>>i >0; i++ {
		result++
	}
	return result
}

func mult(a, b byte) byte {
	result := 0
	for i:=0; a>>i > 0; i++ { //iterate over the bits of a
		if a & (1<<i) > 0 { // if current bit is 1
			fmt.Printf("i: %v\t dodatek: %v\n", i, int(b)<<i)
			result ^= int(b)<<i //xor b multiplied by this power of 2 to result
		}
	}

	if result < generator {
		return byte(result)
	}

	len1, len2 := length(result), length(generator)
	fmt.Printf("\n\naaa  %v\t%v\n\n", len1, len2)
	for i:=len1-len2; i > -1; i-- { //while result is not smaller than the generator
		if result & (1<<(i+len2-1)) > 0{ //if current bit is 1
			result ^= generator << i //align divisor with the result and subtract its value
		}
	}
	return byte(result)
}

func main() {
	mult(byte(33), byte(191))
}
