package group

import "math"

type Clusterer struct {
	Threshold float64
}

func (c Clusterer) Label(vectors [][]float64) []int {
	n := len(vectors)
	if n == 0 {
		return nil
	}
	labels := make([]int, n)
	if n < 2 {
		return labels
	}
	dist := cosineMatrix(vectors)
	active := make([]bool, n)
	size := make([]int, n)
	parent := make([]int, n)
	for i := 0; i < n; i++ {
		active[i] = true
		size[i] = 1
		parent[i] = i
	}
	find := func(x int) int {
		root := x
		for parent[root] != root {
			root = parent[root]
		}
		for parent[x] != root {
			next := parent[x]
			parent[x] = root
			x = next
		}
		return root
	}
	nearest := func(a, prefer int) (int, float64) {
		best := -1
		bestD := math.Inf(1)
		for j := 0; j < n; j++ {
			if j == a || !active[j] {
				continue
			}
			d := dist[a][j]
			if d < bestD || (d == bestD && (j == prefer || (best != prefer && j < best))) {
				bestD = d
				best = j
			}
		}
		return best, bestD
	}

	alive := n
	chain := make([]int, 0, n)
	for alive > 1 {
		if len(chain) == 0 {
			for i := 0; i < n; i++ {
				if active[i] {
					chain = append(chain, i)
					break
				}
			}
		}
		a := chain[len(chain)-1]
		prefer := -1
		if len(chain) > 1 {
			prefer = chain[len(chain)-2]
		}
		b, d := nearest(a, prefer)
		if b < 0 {
			break
		}
		if len(chain) > 1 && b == chain[len(chain)-2] {
			if !(d < c.Threshold) {
				break
			}
			na := float64(size[a])
			nb := float64(size[b])
			denom := na + nb
			for k := 0; k < n; k++ {
				if k == a || k == b || !active[k] {
					continue
				}
				dk := (na*dist[a][k] + nb*dist[b][k]) / denom
				dist[a][k] = dk
				dist[k][a] = dk
			}
			size[a] += size[b]
			active[b] = false
			parent[b] = a
			alive--
			chain = chain[:len(chain)-2]
			continue
		}
		chain = append(chain, b)
		if len(chain) > n {
			break
		}
	}
	for i := 0; i < n; i++ {
		labels[i] = find(i)
	}
	return labels
}

func cosineMatrix(vectors [][]float64) [][]float64 {
	n := len(vectors)
	dist := make([][]float64, n)
	for i := range dist {
		dist[i] = make([]float64, n)
	}
	parallelMap(n, func(i int) struct{} {
		for j := i + 1; j < n; j++ {
			dist[i][j] = cosineDistance(vectors[i], vectors[j])
		}
		return struct{}{}
	})
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			dist[j][i] = dist[i][j]
		}
	}
	return dist
}

func cosineDistance(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 1
	}
	sim := dot / (math.Sqrt(na) * math.Sqrt(nb))
	if sim > 1 {
		sim = 1
	} else if sim < -1 {
		sim = -1
	}
	return 1 - sim
}
