package main 
import (
	"fmt"

)

var verbose = true
var prime = 0x11d
var exp_table = make([]byte, 512)
var log_table = make([]byte, 256)

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
			result ^= int(b)<<i //xor b multiplied by this power of 2 to result
		}
	}

	len1, len2 := length(result), length(prime)
	if len1 < len2 {
		return byte(result)
	}
	for i:=len1-len2; i > -1; i-- { //while result is not smaller than the prime
		if result & (1<<(i+len2-1)) > 0{ //if current bit is 1
			result ^= prime << i //align divisor with the result and subtract its value
		}
	}
	return byte(result)
}


//use generator 2 to init log and exp tables
func init_tables() {
	x := byte(1)
	for i:=0; i<255; i++ {
		exp_table[i] = x
		log_table[x] = byte(i)
		x = mult(x, 2)
	}
	for i:=255; i<512; i++ {
		exp_table[i] = exp_table[i-255]
	}
}

func table_mult(a, b byte) byte {
	if a==0 || b==0 {
		return 0
	}
	return exp_table[int(log_table[a]) + int(log_table[b])]
}


func main() {
	init_tables()
	fmt.Println(table_mult(byte(33), byte(191)))
	fmt.Println(mult(byte(33), byte(191)))
}
