package main

import (
	"context"
	"testing"
)

func BenchmarkWithValue(b *testing.B) {
	for i := 0; i < b.N; i++ {
		aa := "aa"
		bb := "bb"
		cc := "cc"
		dd := "dd"
		ee := "ee"
		ctx1 := context.WithValue(context.TODO(), aa, aa)
		ctx2 := context.WithValue(ctx1, bb, bb)
		ctx3 := context.WithValue(ctx2, cc, cc)
		ctx4 := context.WithValue(ctx3, dd, dd)
		ctx5 := context.WithValue(ctx4, ee, ee)

		_ = ctx1
		_ = ctx2
		_ = ctx3
		_ = ctx4
		_ = ctx5

	}
}

type test struct {
	aa string
	bb string
	cc string
	dd string
	ee string
}

func BenchmarkWithValue_One(b *testing.B) {
	for i := 0; i < b.N; i++ {
		aa := "aa"
		bb := "bb"
		cc := "cc"
		dd := "dd"
		ee := "ee"
		_ = context.WithValue(context.TODO(), "aa", test{aa, bb, cc, dd, ee})
	}
}
