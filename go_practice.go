//https://tour.golang.org/
package main

import (
  "fmt"
  "math/rand"
  "math"
  "runtime"
  "time"
)
var c, python, java bool  //initialized as false
var i, j int = 1,2
const Pi = 3.14  //no need for type
const (
	// Create a huge number by shifting a 1 bit left 100 places.
	// In other words, the binary number that is 1 followed by 100 zeroes.
	Big = 1 << 100
	// Shift it right again 99 places, so we end up with 1<<1, or 2.
	Small = Big >> 99
)
func main() {
  fmt.Println("Happy", Pi, "Day")
  //Happy 3.14 Day     with space!
  fmt.Println("Happy" + "Day")
  //HappyDay     no space!
  var i int  //initialized as 0
  fmt.Println(i, c, python, java)
  var c, python, java = true, false, "no!"
  fmt.Println(i, j , c, python, java)
  fmt.Println("My favorite number is", rand.Intn(10))
  fmt.Printf("Now you have %g problems.", math.Sqrt(7))
  //exported name
  fmt.Println(math.Pi)
  fmt.Println(add(42, 13))
  // := can be used as in place of var/func (but not const)...inside a function,
  a, b := swap("hello", "world")
  fmt.Println(a,b)
  fmt.Println(split(17))
  k := 3  //var k int = 3
  fmt.Println(i, j, k, c, python, java)
  // MaxInt uint64 = 1<<64 - 1
  // fmt.Printf("Type: %T Value: %v\n", MaxInt, MaxInt)
  //Type: uint64 Value: 18446744073709551615
  //var i int
	var f float64
	//var b bool
	var s string
	fmt.Printf("%v %v %v %q\n", i, f, b, s)  //%q a single-quoted character
  //0, 0, false, ""
  //conversion
  x, y := 3, 4
	var n float64 = math.Sqrt(float64(x*x + y*y))
  fmt.Println(x, y, n)
  v := 42 // change me!
	fmt.Printf("v is of type %T\n", v)
  fmt.Println(needInt(Small))
	fmt.Println(needFloat(Small))
	fmt.Println(needFloat(Big))
  sum2:= 0
  for i := 0; i < 10; i++{
    sum2 += i
  }
  fmt.Println(sum2)
  sum3 := 1
  //The init and post statement are optional.
  for ; sum3 < 100; {
    sum3 += sum3
  }
  fmt.Println(sum3)  //1024
  sum4 := 1
  //while loop
  for sum4 < 1000 {
    sum4 += sum4
  }
  fmt.Println(sum4)
  fmt.Println(sqrt(2), sqrt(-4))
  fmt.Println(
    pow(3,2,10),
    pow(3,3,20),   //a second comma
  )
  //switch statement
  switch os := runtime.GOOS; os{
  case "darwin":
    fmt.Println("OS X.")
  case "linux":
    fmt.Println("Linux")
  default:
    fmt.Printf("%s", os)  //"OS X."
  }
  t := time.Now()
  switch{
  case t.Hour() < 12:
    fmt.Println("morning")
  case t.Hour() < 17:
    fmt.Println("afternoon")
  default:
    fmt.Println("evening")
  }
  fmt.Println("when is saturday?")
  today := time.Now().Weekday()
  switch time.Saturday{
  case today + 0:
    fmt.Println("today")
  case today + 1:
    fmt.Println("tomorrow")
  default:
    fmt.Println("too far away")
  }
  //stacking defer
  //A defer statement defers the execution of a function until the surrounding function returns.
  defer fmt.Println("world")
	fmt.Println("hello")
  fmt.Println("counting")
  for i:=0; i < 10; i++{
    defer fmt.Println(i)
  }
  fmt.Println("done")
//   hello
// counting
// done
// 9
// 8
// 7
// 6
// 5
// 4
// 3
// 2
// 1
// 0
// world

}
func pow(x, n, lim float64) float64{
  //a small statement before if condition
  if v := math.Pow(x, n); v < lim{
    return v
  }else{
    return lim
  }
}
func sqrt(x float64) string{
  if x < 0{
    return sqrt(-x) + "i";
  }
  //Sprint formats using the default formats for its operands and returns the resulting string.
  //Spaces are added between operands when neither is a string.
  //Sprintf, Springln, Sprint
  return fmt.Sprint(math.Sqrt(x))
}
func needInt(x int) int {
  return x*10 + 1
}
func needFloat(x float64) float64 {
	return x * 0.1
}
// func add(x int, y int) int {
// 	return x + y
// }
func add(x, y int) int {
	return x + y
}
//named return
func swap(x, y string) (string, string){
  return y, x
}
//naked return
func split(sum int)(x, y int){
  x = sum * 4 / 9
  y = sum - x
  return
}
//type
// bool
// string
// int  int8  int16  int32  int64
// uint uint8 uint16 uint32 uint64 uintptr
// byte // alias for uint8
// rune // alias for int32
//      // represents a Unicode code point
// float32 float64
// complex64 complex128
