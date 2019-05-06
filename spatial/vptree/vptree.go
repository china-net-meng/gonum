// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vptree

import (
	"container/heap"
	"math"
	"sort"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/stat"
)

// Comparable is the element interface for values stored in a vp tree.
type Comparable interface {
	// Distance returns the Euclidean distance between the receiver and
	// the parameter.
	Distance(Comparable) float64
}

// Point represents a point in a k-d space that satisfies the Comparable interface.
type Point []float64

// Distance returns the Euclidean distance between c and the receiver. The concrete
// type of c must be Point.
func (p Point) Distance(c Comparable) float64 {
	q := c.(Point)
	var sum float64
	for dim, c := range p {
		d := c - q[dim]
		sum += d * d
	}
	return math.Sqrt(sum)
}

// Node holds a single point value in a vantage point tree.
type Node struct {
	Point   Comparable
	Radius  float64
	Closer  *Node
	Further *Node
}

// Tree implements a vantage point tree creation and nearest neighbor search.
type Tree struct {
	Root  *Node
	Count int
}

// New returns a vantage point tree constructed from the values in p. The effort
// parameter specifies how much work should be put into optimizing the choice of
// vantage point. If effort is one or less, random vantage points are chosen.
// The order of elements in p will be altered after New returns.
func New(p []Comparable, effort int) *Tree {
	b := builder{work: make([]float64, max(len(p), effort*effort))}
	return &Tree{
		Root:  b.build(p, effort),
		Count: len(p),
	}
}

// builder performs vp-tree construction as described for the simple vp-tree
// algorithm in http://pnylab.com/papers/vptree/vptree.pdf.
type builder struct {
	work []float64
}

func (b *builder) build(s []Comparable, effort int) *Node {
	if len(s) <= 1 {
		if len(s) == 0 {
			return nil
		}
		return &Node{Point: s[0]}
	}
	n := Node{Point: b.selectVantage(s, effort)}
	radius, closer, further := b.partition(n.Point, s)
	n.Radius = radius
	n.Closer = b.build(closer, effort)
	n.Further = b.build(further, effort)
	return &n
}

func (b *builder) selectVantage(s []Comparable, effort int) Comparable {
	if effort <= 1 {
		return s[rand.Intn(len(s))]
	}
	if effort > len(s) {
		effort = len(s)
	}
	var best Comparable
	var bestVar float64
	b.work = b.work[:effort*effort]
	for i, p := range random(effort, s) {
		for j, q := range random(effort, s) {
			b.work[i*effort+j] = p.Distance(q)
		}
		variance := stat.Variance(b.work, nil)
		if variance > bestVar {
			bestVar = variance
			best = p
		}
	}
	return best
}

func random(n int, s []Comparable) []Comparable {
	if n >= len(s) {
		return s
	}
	rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
	return s[:n]
}

func (b *builder) partition(v Comparable, s []Comparable) (radius float64, closer, further []Comparable) {
	b.work = b.work[:len(s)]
	for i, p := range s {
		b.work[i] = v.Distance(p)
	}
	sort.Sort(byDist{dists: b.work, points: s})

	// Note that this does not conform exactly to the description
	// in the paper which specifies d(p, s) < mu for L; in cases
	// where the median element has a lower indexed element with
	// the same distance from the vantage point, L will include a
	// d(p, s) == mu.
	// The additional work required to satisfy the algorithm is
	// not worth doing as it has no effect on the correctness or
	// performance of the algorithm.
	radius = b.work[len(b.work)/2]

	if len(b.work) > 1 {
		// Remove vantage if it is present.
		closer = s[1 : len(b.work)/2]
	}
	further = s[len(b.work)/2:]
	return radius, closer, further
}

type byDist struct {
	dists  []float64
	points []Comparable
}

func (c byDist) Len() int           { return len(c.dists) }
func (c byDist) Less(i, j int) bool { return c.dists[i] < c.dists[j] }
func (c byDist) Swap(i, j int) {
	c.dists[i], c.dists[j] = c.dists[j], c.dists[i]
	c.points[i], c.points[j] = c.points[j], c.points[i]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Len returns the number of elements in the tree.
func (t *Tree) Len() int { return t.Count }

var inf = math.Inf(1)

// Nearest returns the nearest value to the query and the distance between them.
func (t *Tree) Nearest(q Comparable) (Comparable, float64) {
	if t.Root == nil {
		return nil, inf
	}
	n, dist := t.Root.search(q, inf)
	if n == nil {
		return nil, inf
	}
	return n.Point, dist
}

func (n *Node) search(q Comparable, dist float64) (*Node, float64) {
	if n == nil {
		return nil, inf
	}

	d := q.Distance(n.Point)
	dist = math.Min(dist, d)

	bn := n
	if d < n.Radius {
		if d-dist <= n.Radius {
			cn, cd := n.Closer.search(q, dist)
			if cd < dist {
				dist = cd
				bn = cn
			}
		}
		if d+dist >= n.Radius {
			fn, fd := n.Further.search(q, dist)
			if fd < dist {
				dist = fd
				bn = fn
			}
		}
	} else {
		if d+dist >= n.Radius {
			fn, fd := n.Further.search(q, dist)
			if fd < dist {
				dist = fd
				bn = fn
			}
		}
		if d-dist <= n.Radius {
			cn, cd := n.Closer.search(q, dist)
			if cd < dist {
				dist = cd
				bn = cn
			}
		}
	}

	return bn, dist
}

// ComparableDist holds a Comparable and a distance to a specific query. A nil Comparable
// is used to mark the end of the heap, so clients should not store nil values except for
// this purpose.
type ComparableDist struct {
	Comparable Comparable
	Dist       float64
}

// Heap is a max heap sorted on Dist.
type Heap []ComparableDist

func (h *Heap) Max() ComparableDist  { return (*h)[0] }
func (h *Heap) Len() int             { return len(*h) }
func (h *Heap) Less(i, j int) bool   { return (*h)[i].Comparable == nil || (*h)[i].Dist > (*h)[j].Dist }
func (h *Heap) Swap(i, j int)        { (*h)[i], (*h)[j] = (*h)[j], (*h)[i] }
func (h *Heap) Push(x interface{})   { (*h) = append(*h, x.(ComparableDist)) }
func (h *Heap) Pop() (i interface{}) { i, *h = (*h)[len(*h)-1], (*h)[:len(*h)-1]; return i }

// NKeeper is a Keeper that retains the n best ComparableDists that have been passed to Keep.
type NKeeper struct {
	Heap
}

// NewNKeeper returns an NKeeper with the max value of the heap set to infinite distance. The
// returned NKeeper is able to retain at most n values.
func NewNKeeper(n int) *NKeeper {
	k := NKeeper{make(Heap, 1, n)}
	k.Heap[0].Dist = inf
	return &k
}

// Keep adds c to the heap if its distance is less than the maximum value of the heap. If adding
// c would increase the size of the heap beyond the initial maximum length, the maximum value of
// the heap is dropped.
func (k *NKeeper) Keep(c ComparableDist) {
	if c.Dist <= k.Heap[0].Dist { // Favour later finds to displace sentinel.
		if len(k.Heap) == cap(k.Heap) {
			heap.Pop(k)
		}
		heap.Push(k, c)
	}
}

// DistKeeper is a Keeper that retains the ComparableDists within the specified distance of the
// query that it is called to Keep.
type DistKeeper struct {
	Heap
}

// NewDistKeeper returns an DistKeeper with the maximum value of the heap set to d.
func NewDistKeeper(d float64) *DistKeeper { return &DistKeeper{Heap{{Dist: d}}} }

// Keep adds c to the heap if its distance is less than or equal to the max value of the heap.
func (k *DistKeeper) Keep(c ComparableDist) {
	if c.Dist <= k.Heap[0].Dist {
		heap.Push(k, c)
	}
}

// Keeper implements a conditional max heap sorted on the Dist field of the ComparableDist type.
// kd search is guided by the distance stored in the max value of the heap.
type Keeper interface {
	Keep(ComparableDist) // Keep conditionally pushes the provided ComparableDist onto the heap.
	Max() ComparableDist // Max returns the maximum element of the Keeper.
	heap.Interface
}

// NearestSet finds the nearest values to the query accepted by the provided Keeper, k.
// k must be able to return a ComparableDist specifying the maximum acceptable distance
// when Max() is called, and retains the results of the search in min sorted order after
// the call to NearestSet returns.
// If a sentinel ComparableDist with a nil Comparable is used by the Keeper to mark the
// maximum distance, NearestSet will remove it before returning.
func (t *Tree) NearestSet(k Keeper, q Comparable) {
	if t.Root == nil {
		return
	}
	t.Root.searchSet(q, k)

	// Check whether we have retained a sentinel
	// and flag removal if we have.
	removeSentinel := k.Len() != 0 && k.Max().Comparable == nil

	sort.Sort(sort.Reverse(k))

	// This abuses the interface to drop the max.
	// It is reasonable to do this because we know
	// that the maximum value will now be at element
	// zero, which is removed by the Pop method.
	if removeSentinel {
		k.Pop()
	}
}

func (n *Node) searchSet(q Comparable, k Keeper) {
	if n == nil {
		return
	}

	k.Keep(ComparableDist{Comparable: n.Point, Dist: q.Distance(n.Point)})

	d := q.Distance(n.Point)
	dist := k.Max().Dist
	if d < n.Radius {
		if d-dist <= n.Radius {
			n.Closer.searchSet(q, k)
		}
		if d+dist >= n.Radius {
			n.Further.searchSet(q, k)
		}
	} else {
		if d+dist >= n.Radius {
			n.Further.searchSet(q, k)
		}
		if d-dist <= n.Radius {
			n.Closer.searchSet(q, k)
		}
	}
}

// Operation is a function that operates on a Comparable. The bounding volume and tree depth
// of the point is also provided. If done is returned true, the Operation is indicating that no
// further work needs to be done and so the Do function should traverse no further.
type Operation func(Comparable, int) (done bool)

// Do performs fn on all values stored in the tree. A boolean is returned indicating whether the
// Do traversal was interrupted by an Operation returning true. If fn alters stored values' sort
// relationships, future tree operation behaviors are undefined.
func (t *Tree) Do(fn Operation) bool {
	if t.Root == nil {
		return false
	}
	return t.Root.do(fn, 0)
}

func (n *Node) do(fn Operation, depth int) (done bool) {
	if n.Closer != nil {
		done = n.Closer.do(fn, depth+1)
		if done {
			return
		}
	}
	done = fn(n.Point, depth)
	if done {
		return
	}
	if n.Further != nil {
		done = n.Further.do(fn, depth+1)
	}
	return
}
