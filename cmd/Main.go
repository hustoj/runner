package main

import (
    "fmt"
    "time"
)

func main() {
    var a, b int
    fmt.Scanf("%d %d", &a, &b)
    fmt.Println("hello world")
    fmt.Println(a + b)
    time.Sleep(3 * time.Second)
}
