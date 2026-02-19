package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// ================================================================
// Vector Math
// ================================================================

type V3 struct{ X, Y, Z float64 }

func (a V3) Add(b V3) V3      { return V3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a V3) Sub(b V3) V3      { return V3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a V3) Mul(s float64) V3 { return V3{a.X * s, a.Y * s, a.Z * s} }
func (a V3) Had(b V3) V3      { return V3{a.X * b.X, a.Y * b.Y, a.Z * b.Z} }
func (a V3) Dot(b V3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }

func (a V3) Cross(b V3) V3 {
	return V3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}

func (a V3) Len() float64  { return math.Sqrt(a.Dot(a)) }
func (a V3) Norm() V3      { return a.Mul(1.0 / a.Len()) }
func (a V3) Max(b V3) V3   { return V3{math.Max(a.X, b.X), math.Max(a.Y, b.Y), math.Max(a.Z, b.Z)} }

// ================================================================
// Ray, Hit, Material
// ================================================================

type Ray struct{ O, D V3 }

type Material struct {
	Albedo   V3
	Emissive V3
}

type Hit struct {
	T   float64
	P   V3
	N   V3
	Mat *Material
}

// ================================================================
// AABB (Axis-Aligned Bounding Box)
// ================================================================

type Box struct {
	Min, Max V3
	Mat      Material
}

func (b *Box) Hit(r Ray, tMin, tMax float64) (Hit, bool) {
	tNear := tMin
	tFar := tMax
	var nearAxis int
	var nearSide float64

	for axis := 0; axis < 3; axis++ {
		var o, d, lo, hi float64
		switch axis {
		case 0:
			o, d, lo, hi = r.O.X, r.D.X, b.Min.X, b.Max.X
		case 1:
			o, d, lo, hi = r.O.Y, r.D.Y, b.Min.Y, b.Max.Y
		case 2:
			o, d, lo, hi = r.O.Z, r.D.Z, b.Min.Z, b.Max.Z
		}

		if math.Abs(d) < 1e-8 {
			if o < lo || o > hi {
				return Hit{}, false
			}
			continue
		}

		invD := 1.0 / d
		t1 := (lo - o) * invD
		t2 := (hi - o) * invD

		side := -1.0
		if t1 > t2 {
			t1, t2 = t2, t1
			side = 1.0
		}

		if t1 > tNear {
			tNear = t1
			nearAxis = axis
			nearSide = side
		}
		if t2 < tFar {
			tFar = t2
		}

		if tNear > tFar || tFar < tMin {
			return Hit{}, false
		}
	}

	if tNear < tMin {
		return Hit{}, false
	}

	p := r.O.Add(r.D.Mul(tNear))
	var n V3
	switch nearAxis {
	case 0:
		n = V3{nearSide, 0, 0}
	case 1:
		n = V3{0, nearSide, 0}
	case 2:
		n = V3{0, 0, nearSide}
	}

	return Hit{T: tNear, P: p, N: n, Mat: &b.Mat}, true
}

// ================================================================
// BVH (Bounding Volume Hierarchy)
// ================================================================

type BVHNode struct {
	Bounds      Box
	Left, Right *BVHNode
	Objects     []*Box // leaf only
}

func buildBVH(boxes []*Box, depth int) *BVHNode {
	if len(boxes) <= 4 || depth > 20 {
		b := boundingBox(boxes)
		return &BVHNode{Bounds: b, Objects: boxes}
	}

	// Find longest axis
	b := boundingBox(boxes)
	ext := b.Max.Sub(b.Min)
	axis := 0
	if ext.Y > ext.X && ext.Y > ext.Z {
		axis = 1
	} else if ext.Z > ext.X {
		axis = 2
	}

	// Sort by axis center
	mid := len(boxes) / 2
	// Simple median split
	sortBoxes(boxes, axis)

	return &BVHNode{
		Bounds: b,
		Left:   buildBVH(boxes[:mid], depth+1),
		Right:  buildBVH(boxes[mid:], depth+1),
	}
}

func sortBoxes(boxes []*Box, axis int) {
	// Simple insertion sort (fine for our sizes)
	for i := 1; i < len(boxes); i++ {
		key := boxes[i]
		kc := boxCenter(key, axis)
		j := i - 1
		for j >= 0 && boxCenter(boxes[j], axis) > kc {
			boxes[j+1] = boxes[j]
			j--
		}
		boxes[j+1] = key
	}
}

func boxCenter(b *Box, axis int) float64 {
	switch axis {
	case 0:
		return (b.Min.X + b.Max.X) * 0.5
	case 1:
		return (b.Min.Y + b.Max.Y) * 0.5
	default:
		return (b.Min.Z + b.Max.Z) * 0.5
	}
}

func boundingBox(boxes []*Box) Box {
	if len(boxes) == 0 {
		return Box{}
	}
	min := boxes[0].Min
	max := boxes[0].Max
	for _, b := range boxes[1:] {
		min = V3{math.Min(min.X, b.Min.X), math.Min(min.Y, b.Min.Y), math.Min(min.Z, b.Min.Z)}
		max = V3{math.Max(max.X, b.Max.X), math.Max(max.Y, b.Max.Y), math.Max(max.Z, b.Max.Z)}
	}
	return Box{Min: min, Max: max}
}

func (n *BVHNode) Hit(r Ray, tMin, tMax float64) (Hit, bool) {
	if _, ok := n.Bounds.Hit(r, tMin, tMax); !ok {
		return Hit{}, false
	}

	if n.Objects != nil {
		// Leaf
		closest := Hit{T: tMax}
		found := false
		for _, obj := range n.Objects {
			if h, ok := obj.Hit(r, tMin, closest.T); ok {
				closest = h
				found = true
			}
		}
		return closest, found
	}

	hL, okL := n.Left.Hit(r, tMin, tMax)
	limit := tMax
	if okL {
		limit = hL.T
	}
	hR, okR := n.Right.Hit(r, tMin, limit)

	if okR {
		return hR, true
	}
	return hL, okL
}

// ================================================================
// Scene
// ================================================================

type Scene struct {
	BVH *BVHNode
}

func (s *Scene) Intersect(r Ray) (Hit, bool) {
	return s.BVH.Hit(r, 0.001, math.MaxFloat64)
}

// ================================================================
// Sampling
// ================================================================

func cosineHemisphere(n V3, rng *rand.Rand) V3 {
	r1 := rng.Float64()
	r2 := rng.Float64()

	phi := 2 * math.Pi * r2
	cosTheta := math.Sqrt(1 - r1)
	sinTheta := math.Sqrt(r1)

	x := sinTheta * math.Cos(phi)
	y := cosTheta
	z := sinTheta * math.Sin(phi)

	// Build orthonormal basis from normal
	var up V3
	if math.Abs(n.Y) < 0.9 {
		up = V3{0, 1, 0}
	} else {
		up = V3{1, 0, 0}
	}
	t := n.Cross(up).Norm()
	b := n.Cross(t)

	return t.Mul(x).Add(n.Mul(y)).Add(b.Mul(z)).Norm()
}

// ================================================================
// Path Tracer
// ================================================================

func skyLight(dir V3) V3 {
	// Area light from upper-left-front
	keyDir := V3{-0.4, 0.85, -0.35}.Norm()
	key := math.Max(0, dir.Dot(keyDir))
	key = math.Pow(key, 16) * 5.0

	// Soft fill from above
	fill := math.Max(0, dir.Y) * 0.25

	// Low ambient
	amb := 0.015

	return V3{
		key*1.0 + fill*0.7 + amb,
		key*0.95 + fill*0.7 + amb,
		key*0.85 + fill*0.8 + amb,
	}
}

// sampleKeyLight returns a jittered direction toward the key light for NEE
func sampleKeyLight(rng *rand.Rand) V3 {
	keyDir := V3{-0.4, 0.85, -0.35}.Norm()
	// Jitter for soft shadows (cone of ~15 degrees)
	jitter := V3{
		(rng.Float64() - 0.5) * 0.25,
		(rng.Float64() - 0.5) * 0.25,
		(rng.Float64() - 0.5) * 0.25,
	}
	return keyDir.Add(jitter).Norm()
}

func trace(scene *Scene, ray Ray, depth int, rng *rand.Rand) V3 {
	if depth <= 0 {
		return V3{}
	}

	hit, ok := scene.Intersect(ray)
	if !ok {
		return skyLight(ray.D)
	}

	result := hit.Mat.Emissive

	// NEE: explicitly sample the key light for direct illumination
	lightDir := sampleKeyLight(rng)
	nDotL := hit.N.Dot(lightDir)
	if nDotL > 0 {
		shadowRay := Ray{hit.P.Add(hit.N.Mul(0.001)), lightDir}
		if _, blocked := scene.Intersect(shadowRay); !blocked {
			lightVal := skyLight(lightDir)
			result = result.Add(hit.Mat.Albedo.Had(lightVal).Mul(nDotL))
		}
	}

	// Indirect bounce for GI (color bleeding, ambient)
	bounceDir := cosineHemisphere(hit.N, rng)
	bounceRay := Ray{hit.P.Add(hit.N.Mul(0.001)), bounceDir}
	incoming := trace(scene, bounceRay, depth-1, rng)

	result = result.Add(hit.Mat.Albedo.Had(incoming))
	return result
}

// ================================================================
// Camera (Orthographic)
// ================================================================

type Camera struct {
	Origin  V3
	Forward V3
	Right   V3
	Up      V3
	Width   float64
	Height  float64
}

func newCamera(from, to V3, width, height float64) Camera {
	fwd := to.Sub(from).Norm()
	worldUp := V3{0, 1, 0}
	right := fwd.Cross(worldUp).Norm()
	up := right.Cross(fwd).Norm()

	return Camera{
		Origin: from, Forward: fwd,
		Right: right, Up: up,
		Width: width, Height: height,
	}
}

func (c Camera) Ray(u, v float64) Ray {
	o := c.Origin.Add(c.Right.Mul(u * c.Width)).Add(c.Up.Mul(v * c.Height))
	return Ray{O: o, D: c.Forward}
}

// ================================================================
// Scene Building
// ================================================================

func buildSceneFromGrid(grid [][]ContributionDay) (*Scene, V3) {
	cubeSize := 1.0
	gap := 0.25 // doubled gap between tiles
	spacing := cubeSize + gap

	shearZ := 1.0
	heightPerCommit := 0.06
	minHeight := 0.4

	numWeeks := len(grid)

	maxCount := 0
	maxW, maxD := 0, 0
	for w, week := range grid {
		for d, day := range week {
			if day.Count > maxCount {
				maxCount = day.Count
				maxW = w
				maxD = d
			}
		}
	}
	fmt.Fprintf(os.Stderr, "Max daily commits: %d\n", maxCount)

	var boxes []Box
	var minX, maxX, minZ, maxZ float64
	minX, minZ = math.MaxFloat64, math.MaxFloat64
	maxX, maxZ = -math.MaxFloat64, -math.MaxFloat64

	for w := 0; w < numWeeks; w++ {
		for d := 0; d < len(grid[w]); d++ {
			day := grid[w][d]
			count := day.Count

			x := float64(w) * spacing
			z := (float64(d) - float64(w)*shearZ) * spacing

			if x < minX { minX = x }
			if x+cubeSize > maxX { maxX = x + cubeSize }
			if z < minZ { minZ = z }
			if z+cubeSize > maxZ { maxZ = z + cubeSize }

			if count == 0 {
				boxes = append(boxes, Box{
					Min: V3{x, -0.06, z},
					Max: V3{x + cubeSize, 0.0, z + cubeSize},
					Mat: Material{Albedo: V3{0.08, 0.08, 0.08}},
				})
				continue
			}

			t := math.Min(float64(count)/100.0, 1.0)
			albedo := V3{
				0.02 + t*0.16,
				0.12 + t*0.63,
				0.06 + t*0.22,
			}

			// Stacked tiers: 1-100 reflective, 101-200 glow 25%, 201-300 glow 50%, 301+ glow 75%
			tiers := []struct {
				threshold int
				intensity float64
			}{
				{100, 0},    // reflective
				{200, 0.25}, // 25% glow
				{300, 0.50}, // 50% glow
				{math.MaxInt32, 0.75}, // 75% glow
			}

			floorY := 0.0
			remaining := count
			for _, tier := range tiers {
				if remaining <= 0 {
					break
				}
				tierCount := tier.threshold - (count - remaining)
				if tierCount > remaining {
					tierCount = remaining
				}
				tierH := float64(tierCount) * heightPerCommit
				if floorY == 0 && tierH < minHeight {
					tierH = minHeight
				}

				if tier.intensity == 0 {
					// Reflective
					boxes = append(boxes, Box{
						Min: V3{x, floorY, z},
						Max: V3{x + cubeSize, floorY + tierH, z + cubeSize},
						Mat: Material{Albedo: albedo},
					})
				} else {
					// Emissive
					boxes = append(boxes, Box{
						Min: V3{x, floorY, z},
						Max: V3{x + cubeSize, floorY + tierH, z + cubeSize},
						Mat: Material{
							Albedo:   V3{0.15, 0.70, 0.25},
							Emissive: V3{tier.intensity * 0.3, tier.intensity * 1.2, tier.intensity * 0.4},
						},
					})
				}

				floorY += tierH
				remaining -= tierCount
			}
		}
	}

	// Lighthouse beacon on the very top of the tallest tower
	lhX := float64(maxW) * spacing
	lhZ := (float64(maxD) - float64(maxW)*shearZ) * spacing
	lhH := math.Max(minHeight, float64(maxCount)*heightPerCommit)
	beaconH := 0.5
	boxes = append(boxes, Box{
		Min: V3{lhX + 0.1, lhH, lhZ + 0.1},
		Max: V3{lhX + cubeSize - 0.1, lhH + beaconH, lhZ + cubeSize - 0.1},
		Mat: Material{
			Albedo:   V3{0.5, 1.0, 0.6},
			Emissive: V3{1.5, 5.0, 2.0},
		},
	})

	// Per-week ground strips
	for w := 0; w < numWeeks; w++ {
		x := float64(w) * spacing
		zStart := (0 - float64(w)*shearZ) * spacing
		zEnd := (6-float64(w)*shearZ)*spacing + cubeSize
		boxes = append(boxes, Box{
			Min: V3{x - 0.05, -0.25, zStart - 0.05},
			Max: V3{x + cubeSize + 0.05, -0.06, zEnd + 0.05},
			Mat: Material{Albedo: V3{0.10, 0.10, 0.12}},
		})
	}

	ptrs := make([]*Box, len(boxes))
	for i := range boxes {
		ptrs[i] = &boxes[i]
	}
	bvh := buildBVH(ptrs, 0)

	maxH := float64(maxCount) * heightPerCommit
	center := V3{(minX + maxX) / 2, maxH / 3, (minZ + maxZ) / 2}
	return &Scene{BVH: bvh}, center
}

// ================================================================
// Rendering
// ================================================================

func render(scene *Scene, cam Camera, imgW, imgH, samples, bounces int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))

	// Stratified sampling: divide samples into a grid
	sqrtN := int(math.Ceil(math.Sqrt(float64(samples))))
	actualSamples := sqrtN * sqrtN

	var done int64
	total := int64(imgH)
	numCPU := runtime.NumCPU()
	var wg sync.WaitGroup

	rowCh := make(chan int, imgH)
	for y := 0; y < imgH; y++ {
		rowCh <- y
	}
	close(rowCh)

	for cpu := 0; cpu < numCPU; cpu++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewSource(rand.Int63()))

			for y := range rowCh {
				for x := 0; x < imgW; x++ {
					var col V3
					var alpha float64

					// Stratified: sample one point per grid cell
					for sy := 0; sy < sqrtN; sy++ {
						for sx := 0; sx < sqrtN; sx++ {
							jx := (float64(sx) + rng.Float64()) / float64(sqrtN)
							jy := (float64(sy) + rng.Float64()) / float64(sqrtN)

							u := (float64(x) + jx - float64(imgW)*0.5) / float64(imgW)
							v := (float64(imgH)*0.5 - float64(y) - jy) / float64(imgH)

							ray := cam.Ray(u, v)

							// Check if primary ray hits geometry (for alpha)
							_, hit := scene.Intersect(ray)
							if hit {
								alpha += 1.0
							}

							sample := trace(scene, ray, bounces, rng)

							maxC := 8.0
							sample = V3{
								math.Min(sample.X, maxC),
								math.Min(sample.Y, maxC),
								math.Min(sample.Z, maxC),
							}

							col = col.Add(sample)
						}
					}
					col = col.Mul(1.0 / float64(actualSamples))
					alpha /= float64(actualSamples)

					// ACES-ish tone mapping
					col = V3{
						acesTone(col.X),
						acesTone(col.Y),
						acesTone(col.Z),
					}

					// Gamma
					col = V3{
						math.Pow(clampF(col.X), 1.0/2.2),
						math.Pow(clampF(col.Y), 1.0/2.2),
						math.Pow(clampF(col.Z), 1.0/2.2),
					}

					a := uint8(clampF(alpha) * 255)
					img.SetRGBA(x, y, color.RGBA{
						R: uint8(clampF(col.X) * float64(a)),
						G: uint8(clampF(col.Y) * float64(a)),
						B: uint8(clampF(col.Z) * float64(a)),
						A: a,
					})
				}

				n := atomic.AddInt64(&done, 1)
				if n%(total/20+1) == 0 || n == total {
					fmt.Fprintf(os.Stderr, "\rRendering: %d%%", n*100/total)
				}
			}
		}()
	}

	wg.Wait()
	fmt.Fprintln(os.Stderr)
	return img
}

func acesTone(x float64) float64 {
	// Simplified ACES filmic tone mapping
	a := x * (2.51*x + 0.03)
	b := x*(2.43*x+0.59) + 0.14
	return a / b
}

func clampF(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// ================================================================
// GitHub API
// ================================================================

type ContributionDay struct {
	Count int
	Level int
}

type gqlResponse struct {
	Data struct {
		User struct {
			ContributionsCollection struct {
				ContributionCalendar struct {
					TotalContributions int `json:"totalContributions"`
					Weeks              []struct {
						ContributionDays []struct {
							ContributionCount int    `json:"contributionCount"`
							Color             string `json:"color"`
							Date              string `json:"date"`
							Weekday           int    `json:"weekday"`
						} `json:"contributionDays"`
					} `json:"weeks"`
				} `json:"contributionCalendar"`
			} `json:"contributionsCollection"`
		} `json:"user"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func getToken() (string, error) {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t, nil
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("set GITHUB_TOKEN or install gh CLI: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func colorToLevel(c string) int {
	switch strings.ToLower(c) {
	case "#9be9a8", "#0e4429":
		return 1
	case "#40c463", "#006d32":
		return 2
	case "#30a14e", "#26a641":
		return 3
	case "#216e39", "#39d353":
		return 4
	default:
		return 0
	}
}

func fetchContributions(username, token string) ([][]ContributionDay, int, error) {
	query := fmt.Sprintf(`{"query":"{ user(login: \"%s\") { contributionsCollection { contributionCalendar { totalContributions weeks { contributionDays { contributionCount color date weekday } } } } } }"}`, username)

	req, err := http.NewRequest("POST", "https://api.github.com/graphql", strings.NewReader(query))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var result gqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, err
	}
	if len(result.Errors) > 0 {
		return nil, 0, fmt.Errorf("graphql: %s", result.Errors[0].Message)
	}

	cal := result.Data.User.ContributionsCollection.ContributionCalendar
	weeks := cal.Weeks
	grid := make([][]ContributionDay, len(weeks))
	for w, week := range weeks {
		grid[w] = make([]ContributionDay, len(week.ContributionDays))
		for d, day := range week.ContributionDays {
			grid[w][d] = ContributionDay{
				Count: day.ContributionCount,
				Level: colorToLevel(day.Color),
			}
		}
	}
	return grid, cal.TotalContributions, nil
}

// ================================================================
// Flat-Shaded Isometric Renderer
// ================================================================

func renderFlatIso(grid [][]ContributionDay) *image.RGBA {
	tw := 18.0 // tile half-width
	th := 9.0  // tile half-height
	yScale := 10.0
	heightPerCommit := 0.06
	minHeight := 0.5

	// Sheared layout: each week advances 1 in X and retreats shearZ in Z.
	// This makes the band go more horizontal in isometric projection.
	// shearZ=0 → standard diagonal; shearZ=1 → fully horizontal.
	shearZ := 1.0

	maxCount := 0
	for _, week := range grid {
		for _, day := range week {
			if day.Count > maxCount {
				maxCount = day.Count
			}
		}
	}
	numWeeks := len(grid)

	// Compute exact screen-space bounding box by checking all tile corners
	sxMin, sxMax := math.MaxFloat64, -math.MaxFloat64
	syMin, syMax := math.MaxFloat64, -math.MaxFloat64
	for w := 0; w < numWeeks; w++ {
		for d := 0; d < len(grid[w]); d++ {
			wx := float64(w)
			wz := float64(d) - float64(w)*shearZ
			h := 0.0
			if grid[w][d].Count > 0 {
				h = math.Max(minHeight, float64(grid[w][d].Count)*heightPerCommit)
			}
			// Check all 8 corners of the cube (4 bottom + 4 top)
			for _, dx := range []float64{0, 1} {
				for _, dz := range []float64{0, 1} {
					px, pz := wx+dx, wz+dz
					for _, py := range []float64{0, h} {
						sx := (px - pz) * tw
						sy := (px+pz)*th - py*yScale
						if sx < sxMin { sxMin = sx }
						if sx > sxMax { sxMax = sx }
						if sy < syMin { syMin = sy }
						if sy > syMax { syMax = sy }
					}
				}
			}
		}
	}

	margin := 25.0
	cx := -sxMin + margin
	cy := -syMin + margin
	actualW := int(sxMax-sxMin+margin*2) + 1
	actualH := int(syMax-syMin+margin*2) + 1

	img := image.NewRGBA(image.Rect(0, 0, actualW, actualH))
	// Dark background
	bg := color.RGBA{30, 30, 32, 255}
	for y := 0; y < actualH; y++ {
		for x := 0; x < actualW; x++ {
			img.SetRGBA(x, y, bg)
		}
	}

	project := func(px, py, pz float64) [2]float64 {
		return [2]float64{
			(px-pz)*tw + cx,
			(px+pz)*th - py*yScale + cy,
		}
	}

	// Collect tiles
	type tile struct {
		wx, wz float64
		h      float64
		col    color.RGBA
		sortK  float64
	}

	var tiles []tile
	for w := 0; w < numWeeks; w++ {
		for d := 0; d < len(grid[w]); d++ {
			day := grid[w][d]
			wx := float64(w)
			wz := float64(d) - float64(w)*shearZ

			h := 0.0
			if day.Count > 0 {
				h = math.Max(minHeight, float64(day.Count)*heightPerCommit)
			}

			t := math.Min(float64(day.Count)/100.0, 1.0)
			var c color.RGBA
			if day.Count == 0 {
				c = color.RGBA{40, 42, 44, 255}
			} else {
				c = color.RGBA{
					uint8(5 + t*40),
					uint8(30 + t*160),
					uint8(15 + t*55),
					255,
				}
			}

			tiles = append(tiles, tile{wx, wz, h, c, wx + wz})
		}
	}

	// Two-pass painter's algorithm:
	// Pass 1: all ground-level tiles (so they never clip towers)
	// Pass 2: all elevated tiles, sorted far-to-near
	sort.Slice(tiles, func(i, j int) bool {
		si, sj := tiles[i].sortK, tiles[j].sortK
		if si != sj {
			return si < sj
		}
		return tiles[i].wx < tiles[j].wx
	})

	// Draw a single continuous ground parallelogram instead of individual tiles
	// to avoid sawtooth edges
	numW := float64(numWeeks)
	groundColor := color.RGBA{45, 45, 48, 255}
	groundPts := [][2]float64{
		project(0, 0, 0-0*shearZ),             // back-left (week 0, day 0)
		project(numW, 0, 0-numW*shearZ),        // back-right (last week, day 0)
		project(numW+1, 0, 7-numW*shearZ),      // front-right (last week, day 6 + tile width)
		project(0, 0, 7),                        // front-left (week 0, day 6 + tile width)
	}
	fillPolyF(img, groundPts, groundColor)

	drawTower := func(t tile) {
		x0, z0 := t.wx, t.wz
		x1, z1 := t.wx+1, t.wz+1

		// Top face only — height shown by vertical position
		tf := [][2]float64{
			project(x0, t.h, z0),
			project(x1, t.h, z0),
			project(x1, t.h, z1),
			project(x0, t.h, z1),
		}
		fillPolyF(img, tf, t.col)
	}

	// Draw towers sorted far to near
	var elevated []tile
	for _, t := range tiles {
		if t.h >= 0.01 {
			elevated = append(elevated, t)
		}
	}
	sort.Slice(elevated, func(i, j int) bool {
		si, sj := elevated[i].sortK, elevated[j].sortK
		if si != sj {
			return si < sj
		}
		return elevated[i].wx < elevated[j].wx
	})
	for _, t := range elevated {
		drawTower(t)
	}

	return img
}

func dimColor(c color.RGBA, factor float64) color.RGBA {
	return color.RGBA{
		uint8(float64(c.R) * factor),
		uint8(float64(c.G) * factor),
		uint8(float64(c.B) * factor),
		255,
	}
}

func fillPolyF(img *image.RGBA, pts [][2]float64, c color.RGBA) {
	minY, maxY := pts[0][1], pts[0][1]
	for _, p := range pts {
		if p[1] < minY {
			minY = p[1]
		}
		if p[1] > maxY {
			maxY = p[1]
		}
	}

	iy0 := int(math.Floor(minY))
	iy1 := int(math.Ceil(maxY))
	n := len(pts)
	bounds := img.Bounds()

	for y := iy0; y <= iy1; y++ {
		if y < bounds.Min.Y || y >= bounds.Max.Y {
			continue
		}
		fy := float64(y) + 0.5
		var xs []float64
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			y0, y1 := pts[i][1], pts[j][1]
			if y0 == y1 {
				continue
			}
			if fy < math.Min(y0, y1) || fy >= math.Max(y0, y1) {
				continue
			}
			t := (fy - y0) / (y1 - y0)
			x := pts[i][0] + t*(pts[j][0]-pts[i][0])
			xs = append(xs, x)
		}

		sort.Float64s(xs)
		for k := 0; k+1 < len(xs); k += 2 {
			x0 := int(math.Floor(xs[k]))
			x1 := int(math.Ceil(xs[k+1]))
			for x := x0; x <= x1; x++ {
				if x >= bounds.Min.X && x < bounds.Max.X {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
}

// ================================================================
// Main
// ================================================================

func cropTransparent(img *image.RGBA) *image.RGBA {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y).A > 0 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < minX || maxY < minY {
		return img
	}
	cropped := image.NewRGBA(image.Rect(0, 0, maxX-minX+1, maxY-minY+1))
	for y := minY; y <= maxY; y++ {
		copy(
			cropped.Pix[(y-minY)*cropped.Stride:],
			img.Pix[(y-b.Min.Y)*img.Stride+(minX-b.Min.X)*4:(y-b.Min.Y)*img.Stride+(maxX-b.Min.X+1)*4],
		)
	}
	return cropped
}

func main() {
	user := flag.String("user", "mrjoshuak", "GitHub username")
	out := flag.String("o", "skyline.png", "output file")
	width := flag.Int("w", 1200, "image width")
	height := flag.Int("h", 400, "image height")
	samples := flag.Int("samples", 64, "samples per pixel")
	bounces := flag.Int("bounces", 5, "max ray bounces")
	flat := flag.Bool("flat", false, "flat-shaded isometric (quick layout preview)")
	crop := flag.Bool("crop", false, "trim transparent pixels from output")
	flag.Parse()

	token, err := getToken()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	grid, total, err := fetchContributions(*user, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Fetched %d contributions across %d weeks\n", total, len(grid))

	var img *image.RGBA

	if *flat {
		fmt.Fprintf(os.Stderr, "Rendering flat isometric preview...\n")
		img = renderFlatIso(grid)
	} else {
		scene, center := buildSceneFromGrid(grid)
		fmt.Fprintf(os.Stderr, "Scene built, BVH ready\n")

		// Camera: equal X/Z for isometric, shearZ=1.0 makes layout horizontal
		camDir := V3{-0.50, -0.65, -0.50}.Norm()
		camPos := center.Sub(camDir.Mul(150))
		orthoW := 112.0
		orthoH := orthoW * float64(*height) / float64(*width)
		cam := newCamera(camPos, center, orthoW, orthoH)

		fmt.Fprintf(os.Stderr, "Rendering %dx%d @ %d spp, %d bounces...\n", *width, *height, *samples, *bounces)
		img = render(scene, cam, *width, *height, *samples, *bounces)
	}

	if *crop {
		img = cropTransparent(img)
		fmt.Fprintf(os.Stderr, "Cropped to %dx%d\n", img.Bounds().Dx(), img.Bounds().Dy())
	}

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	png.Encode(f, img)
	fmt.Fprintf(os.Stderr, "Saved %s\n", *out)
}
