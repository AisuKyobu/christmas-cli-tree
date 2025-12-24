package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
	"github.com/gdamore/tcell/v2"
)

// --- 基础配置 ---

const (
	TreeTopMargin = 3   // 树顶距离屏幕上方的距离
	TreeHeight    = 22  // 树的高度
	TreeBaseWidth = 30  // 树底部的最大宽度（半径）
	StarSpeed     = 0.2 // 星星公转速度
	VerticalSpeed = 0.5 // 星星上下移动的频率

	// 可调：星星时间步长（越小移动越慢），你可以在这里自行修改数值
	StarTimeStep = 0.06

	// 可调：星星照亮范围（默认略小于原来 10.0）
	LightRadius = 8.0

	// 粒子轨迹参数：延长轨迹特效（初始生命更长，衰减更慢）
	ParticleInitialLife = 1.2
	ParticleLifeDecay   = 0.05
	// 前面（Z>=0）时额外多生成的粒子数，以使正面轨迹更显眼、更长
	ParticleFrontExtra = 1

	// 天空相关配置
	SkyStarCount    = 8     // 天空中星星的数量（很少）
	SkyGlowRadius   = 2     // 星星周围的微弱发光半径（格子单位）
	SkyBaseR        = 6     // 天空基底颜色 (暗蓝)
	SkyBaseG        = 10
	SkyBaseB        = 40
	SkyTwinkleSpeed = 1.2   // 星星闪烁速度（可调）
)

// --- 类型定义 ---

// Vector3 简单的3D坐标
type Vector3 struct {
	X, Y, Z float64
}

// Particle 粒子结构体
type Particle struct {
	Pos      Vector3
	Velocity Vector3
	Life     float64 // 生命值 0.0 - 1.0
	Color    tcell.Color
}

// CellType 单元格类型
type CellType int

const (
	TypeEmpty CellType = iota
	TypeNeedle
	TypeTrunk
	TypeDecor // 装饰品
	TypeGift  // 礼物
)

// TreeCell 树的单元格信息
type TreeCell struct {
	Type      CellType
	Char      rune
	BaseColor tcell.Color // 原始颜色
	LitColor  tcell.Color // 被照亮后的颜色
	X, Y      int         // 相对于树中心的偏移坐标 (0,0 是树底中心)
}

// SkyStar 表示天空中的一个小星星
type SkyStar struct {
	X, Y  int     // 屏幕坐标
	Phase float64 // 相位用于闪烁差异化
	Speed float64 // 个别闪烁速度微差
}

// --- 全局变量 ---
var (
	particles []Particle
	rnd       = rand.New(rand.NewSource(time.Now().UnixNano()))

	skyStars   []SkyStar
	prevWidth  = -1
	prevTopY   = -1
)

func main() {
	// 1. 初始化 Tcell
	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer screen.Fini()

	// 2. 构建树和礼物的数据 (静态数据，只构建一次)
	treeData := buildRichTreeData()

	// 3. 事件监听
	quit := make(chan struct{})
	go func() {
		for {
			ev := screen.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Rune() == 'q' {
					close(quit)
					return
				}
			case *tcell.EventResize:
				screen.Sync()
			}
		}
	}()

	// 监听系统信号 (Ctrl+C / SIGTERM) 以优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			close(quit)
		case <-quit:
			// already quitting
		}
	}()

	ticker := time.NewTicker(time.Millisecond * 40) // 25 FPS
	defer ticker.Stop()

	t := 0.0 // 时间变量

	// 4. 主循环
	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			screen.Clear()
			width, height := screen.Size()
			
			// 计算屏幕中心
			midX := width / 2
			// 树底部在屏幕的位置 (留出一点底部空间)
			baseY := height - 4 
			// 树顶在屏幕的位置
			topY := baseY - TreeHeight

			// 如果屏幕宽度或树顶位置变化，重建天空星星
			if width != prevWidth || topY != prevTopY {
				initSkyStars(width, topY)
				prevWidth = width
				prevTopY = topY
			}

			// --- SKY: 绘制天空背景与微弱发光星星 ---
			// 绘制天空的基底背景（仅在树上方）
			for y := 0; y < topY; y++ {
				for x := 0; x < width; x++ {
					bg := tcell.NewRGBColor(SkyBaseR, SkyBaseG, SkyBaseB)
					st := tcell.StyleDefault.Background(bg)
					// 使用空格填充背景
					screen.SetContent(x, y, ' ', nil, st)
				}
			}

			// 渲染星星与微弱的光晕（基于时间 t 实现闪烁）
			for _, s := range skyStars {
				phase := s.Phase + t*s.Speed*SkyTwinkleSpeed
				brightness := 0.4 + 0.6*(0.5*(1+math.Sin(phase))) // 0.4 ~ 1.0
				// 星星主体（小点）
				colVal := int32(200 + int32(brightness*55))
				st := tcell.StyleDefault.Foreground(tcell.NewRGBColor(colVal, colVal, colVal)).Background(tcell.ColorReset)
				if s.Y >= 0 && s.Y < height && s.X >= 0 && s.X < width {
					screen.SetContent(s.X, s.Y, '.', nil, st)
				}
				// 光晕：在周围格子稍微提升背景亮度（只在树上方区域生效）
				for dy := -SkyGlowRadius; dy <= SkyGlowRadius; dy++ {
					for dx := -SkyGlowRadius; dx <= SkyGlowRadius; dx++ {
						tx := s.X + dx
						ty := s.Y + dy
						if tx < 0 || tx >= width || ty < 0 || ty >= topY {
							continue
						}
						dist := math.Sqrt(float64(dx*dx+dy*dy))
						if dist > float64(SkyGlowRadius) {
							continue
						}
						// 距离越近亮度越高
						f := (1.0 - dist/float64(SkyGlowRadius)) * brightness * 0.6
						bg := skyBgColor(f)
						st2 := tcell.StyleDefault.Background(bg)
						screen.SetContent(tx, ty, ' ', nil, st2)
					}
				}
			}
			// --- SKY END ---

			// 使用可调时间步长控制速度（原来是固定 t += 0.1）
			t += StarTimeStep

			// --- A. 计算星星的 3D 螺旋轨迹 ---
			
			// 1. 垂直运动 (Y轴): 使用 Sin 函数实现平滑的上下往复
			// 范围从 0 (树顶) 到 1 (接近树干)
			// (Sin(t) + 1) / 2 将 -1~1 映射到 0~1
			verticalProgress := (math.Sin(t*VerticalSpeed) + 1) / 2
			
			// 稍微调整范围，让它不要完全碰到树底，也不要飞出树顶太远
			// 0.05 ~ 0.95
			clampedProgress := 0.05 + verticalProgress*0.9
			
			starY := float64(topY) + clampedProgress*float64(TreeHeight)

			// 2. 半径计算 (圆锥体): 越往下半径越大
			// 顶部半径很小(1)，底部半径较大(TreeBaseWidth/2 + 余裕)
			currentRadius := 2.0 + clampedProgress*float64(TreeBaseWidth/2+2)

			// 3. 水平运动 (X, Z): 快速旋转
			// 加上 offset 让螺旋线在上升和下降时对称但相位不同，形成好看的交错
			rotateSpeed := t * 3.0
			starRelX := math.Cos(rotateSpeed) * currentRadius
			starRelZ := math.Sin(rotateSpeed) * currentRadius // Z轴：正数在屏幕前，负数在屏幕后

			// 星星的屏幕坐标
			starScreenX := float64(midX) + starRelX * 2.0 // X轴拉伸一下适配终端字符比例(通常高是宽的2倍)
			starScreenY := starY

			// 星星颜色 (彩虹变换)
			starHue := int(t*20) % 360
			starColor := hsvToRgb(float64(starHue), 1.0, 1.0)

			// --- B. 更新粒子系统 ---
			spawnParticles(starScreenX, starScreenY, starRelZ, starColor)
			updateParticles()

			// --- C. 渲染树木与礼物 (应用光照) ---
			
			// 使用顶部常量作为光照半径（便于直接调整）
			lightRadius := LightRadius

			// --- 新增：把树的未使用区域填成与天空一致的背景（横向覆盖整个屏幕） ---
			// 标记所有树单元格占用的位置
			occupied := make(map[[2]int]bool)
			for _, cell := range treeData {
				cellScreenX := midX + cell.X
				cellScreenY := baseY + cell.Y
				if cellScreenX < 0 || cellScreenX >= width || cellScreenY < 0 || cellScreenY >= height {
					continue
				}
				occupied[[2]int{cellScreenX, cellScreenY}] = true
			}
			
			// 标记天空星星及其光晕占用位置，避免被填充覆盖
			skyOccupied := make(map[[2]int]bool)
			for _, s := range skyStars {
				for dy := -SkyGlowRadius; dy <= SkyGlowRadius; dy++ {
					for dx := -SkyGlowRadius; dx <= SkyGlowRadius; dx++ {
						tx := s.X + dx
						ty := s.Y + dy
						if tx < 0 || tx >= width || ty < 0 || ty >= topY {
							continue
						}
						// 只在树上方区域标记
						skyOccupied[[2]int{tx, ty}] = true
					}
				}
				// 标记星星自身
				if s.X >= 0 && s.X < width && s.Y >= 0 && s.Y < topY {
					skyOccupied[[2]int{s.X, s.Y}] = true
				}
			}

			// 填充范围：从树顶到树底附近，横向覆盖整个屏幕（0..width-1）
			left := 0
			right := width - 1
			bottom := baseY + 4 // 覆盖树干和礼物区域
			if bottom >= height {
				bottom = height - 1
			}
			for y := topY; y <= bottom; y++ {
				for x := left; x <= right; x++ {
					if occupied[[2]int{x, y}] || skyOccupied[[2]int{x, y}] {
						continue
					}
					st := tcell.StyleDefault.Background(tcell.NewRGBColor(SkyBaseR, SkyBaseG, SkyBaseB))
					screen.SetContent(x, y, ' ', nil, st)
				}
			}
			// --- 新增结束 ---

			for _, cell := range treeData {
				// 计算该单元格在当前屏幕的绝对位置
				cellScreenX := midX + cell.X
				cellScreenY := baseY + cell.Y // cell.Y 是负数，相对于 base

				// 简单的裁剪，防止画出屏幕
				if cellScreenX < 0 || cellScreenX >= width || cellScreenY < 0 || cellScreenY >= height {
					continue
				}

				// 计算到星星的 2D 距离 (用于光照强度)
				dx := float64(cellScreenX) - starScreenX
				dy := float64(cellScreenY) - starScreenY
				// 修正 X 轴距离权重，因为终端字符非正方形
				dist := math.Sqrt((dx*0.5)*(dx*0.5) + dy*dy)

				// 将树的背景设为与天空一致的暗蓝色
				finalStyle := tcell.StyleDefault.Background(tcell.NewRGBColor(SkyBaseR, SkyBaseG, SkyBaseB))
				
				// 基础绘制字符
				drawChar := cell.Char
				fgColor := cell.BaseColor

				// 光照逻辑
				if dist < lightRadius {
					// 距离越近越亮
					// 对于普通的树叶/干，可以使用 LitColor
					// 但装饰（TypeDecor）和礼物（TypeGift）在被照亮时不改变颜色
					if cell.Type != TypeDecor && cell.Type != TypeGift {
						fgColor = cell.LitColor
					}
					// 注意：装饰不再在被照亮时变成 '★'，也保持原色
				}

				finalStyle = finalStyle.Foreground(fgColor)
				screen.SetContent(cellScreenX, cellScreenY, drawChar, nil, finalStyle)
			}

			// --- D. 渲染粒子 (在星星之前还是之后？) ---
			// 简单的粒子渲染，粒子总是发光的
			for _, p := range particles {
				if p.Pos.X >= 0 && p.Pos.X < float64(width) && p.Pos.Y >= 0 && p.Pos.Y < float64(height) {
					// 使用与天空/树相同的背景，避免覆盖时留下“缺块”
					st := tcell.StyleDefault.Foreground(p.Color).Background(tcell.NewRGBColor(SkyBaseR, SkyBaseG, SkyBaseB))
					screen.SetContent(int(p.Pos.X), int(p.Pos.Y), '.', nil, st)
				}
			}

			// --- E. 渲染星星 ---
			// 如果星星在背面 (Z < 0)，不再绘制（去除“半颗星”的可见效果）
			shouldDrawStar := true
			if starRelZ < 0.0 {
				shouldDrawStar = false
			}

			if shouldDrawStar {
				// 星星也使用相同背景，防止重绘时出现背景闪烁不一致
				st := tcell.StyleDefault.Foreground(starColor).Background(tcell.NewRGBColor(SkyBaseR, SkyBaseG, SkyBaseB)).Bold(true)
				screen.SetContent(int(starScreenX), int(starScreenY), '★', nil, st)
			}

			screen.Show()
		}
	}
}

// --- 辅助逻辑 ---

// spawnParticles 生成拖尾粒子
func spawnParticles(x, y, z float64, color tcell.Color) {
	// 基础数量
	count := rnd.Intn(3) + 2
	// 如果星星在前面，生成更多粒子以延长正面的轨迹特效
	if z >= 0 {
		count += ParticleFrontExtra
	}
	for i := 0; i < count; i++ {
		// 随机散布一点点
		offsetX := (rnd.Float64() - 0.5) * 2.0
		offsetY := (rnd.Float64() - 0.5) * 1.0
		
		particles = append(particles, Particle{
			Pos: Vector3{X: x + offsetX, Y: y + offsetY, Z: z},
			// 粒子稍微向下飘落，前面星星粒子更稳定（更小的速度）以延长视觉停留
			Velocity: Vector3{X: (rnd.Float64() - 0.5) * 0.2, Y: rnd.Float64() * 0.2},
			Life:     ParticleInitialLife,
			Color:    color,
		})
	}
}

// updateParticles 更新粒子状态
func updateParticles() {
	var alive []Particle
	for _, p := range particles {
		p.Pos.X += p.Velocity.X
		p.Pos.Y += p.Velocity.Y
		p.Life -= ParticleLifeDecay // 使用较小的衰减以延长轨迹

		if p.Life > 0 {
			alive = append(alive, p)
		}
	}
	particles = alive
}

// buildRichTreeData 构建更丰富的树和礼物数据
func buildRichTreeData() []TreeCell {
	var cells []TreeCell

	// 1. 构建树叶 (更胖的三角形)
	for y := 0; y < TreeHeight; y++ {
		// y=0 是树顶 (相对于树本身坐标系)
		// 坐标转换：由于屏幕是向下增加，我们这里生成的 cell.Y 应该是负数（相对于树底）
		// 或者我们定义 0 为树底，-TreeHeight 为树顶
		
		currentY := -TreeHeight + y // -22 到 -1
		
		// 宽度线性增加
		width := int(float64(y) / float64(TreeHeight) * float64(TreeBaseWidth))
		if width < 1 { width = 1 }

		for x := -width; x <= width; x++ {
			char := '*'
			baseColor := tcell.ColorGreen
			litColor := tcell.NewRGBColor(100, 255, 100) // 亮绿色

			// 随机装饰品 (从 15% 略微减少到 10%)
			if rnd.Float64() < 0.10 {
				char = getRandomDecorChar()
				baseColor = getRandomDecorColor() // 自身已经发光
				// 装饰在被照亮时不改变颜色
				litColor = baseColor
				cells = append(cells, TreeCell{Type: TypeDecor, Char: char, BaseColor: baseColor, LitColor: litColor, X: x, Y: currentY})
				continue
			}
			
			// 随机预置灯条 (从 5% 略微减少到 3%)
			if rnd.Float64() < 0.03 {
				char = '•'
				baseColor = tcell.NewRGBColor(255, 215, 0) // 金色
				// 灯在被照亮时不改变颜色
				litColor = baseColor
				cells = append(cells, TreeCell{Type: TypeDecor, Char: char, BaseColor: baseColor, LitColor: litColor, X: x, Y: currentY})
				continue
			}

			// 边缘纹理
			if x == -width { char = '/' }
			if x == width { char = '\\' }

			cells = append(cells, TreeCell{Type: TypeNeedle, Char: char, BaseColor: baseColor, LitColor: litColor, X: x, Y: currentY})
		}
	}

	// 2. 构建粗壮的树干
	trunkHeight := 4
	trunkWidth := 5 // 更粗
	for y := 0; y < trunkHeight; y++ {
		for x := -trunkWidth/2; x <= trunkWidth/2; x++ {
			cells = append(cells, TreeCell{
				Type:      TypeTrunk,
				Char:      '#', // 实心的树干
				BaseColor: tcell.NewRGBColor(101, 67, 33), // 深褐色
				LitColor:  tcell.NewRGBColor(200, 150, 50), // 亮褐色
				X:         x,
				Y:         y, // 0 到 3
			})
		}
	}

	// 3. 构建树下的礼物
	// 略微减少礼物数量并且缩小尺寸
	giftConfigs := []struct{ x, w, h int; color tcell.Color }{
		{-8, 3, 2, tcell.ColorRed},
		{6, 4, 2, tcell.ColorBlue},
	}

	for _, g := range giftConfigs {
		for gh := 0; gh < g.h; gh++ {
			for gw := 0; gw < g.w; gw++ {
				char := 'H' // 礼物盒纹理
				if gh == g.h/2 { char = '-' } // 丝带
				if gw == g.w/2 { char = '|' } // 丝带

				cells = append(cells, TreeCell{
					Type:      TypeGift,
					Char:      char,
					BaseColor: g.color,
					// 礼物在被照亮时不改变颜色
					LitColor:  g.color,
					X:         g.x + gw,
					Y:         trunkHeight - gh - 1, // 放在树干底部平面
				})
			}
		}
	}

	return cells
}

func getRandomDecorChar() rune {
	chars := []rune{'o', '@', 'O', '8', '&', '$'}
	return chars[rnd.Intn(len(chars))]
}

func getRandomDecorColor() tcell.Color {
	colors := []tcell.Color{
		tcell.ColorRed,
		tcell.ColorYellow,
		tcell.NewRGBColor(255, 105, 180), // HotPink
		tcell.NewRGBColor(0, 255, 255),   // Cyan
	}
	return colors[rnd.Intn(len(colors))]
}

// hsvToRgb 辅助函数：生成彩虹色
func hsvToRgb(h, s, v float64) tcell.Color {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60.0, 2)-1))
	m := v - c
	var r, g, b float64
	
	switch {
	case 0 <= h && h < 60:
		r, g, b = c, x, 0
	case 60 <= h && h < 120:
		r, g, b = x, c, 0
	case 120 <= h && h < 180:
		r, g, b = 0, c, x
	case 180 <= h && h < 240:
		r, g, b = 0, x, c
	case 240 <= h && h < 300:
		r, g, b = x, 0, c
	case 300 <= h && h < 360:
		r, g, b = c, 0, x
	}
	
	R := int32((r + m) * 255)
	G := int32((g + m) * 255)
	B := int32((b + m) * 255)
	return tcell.NewRGBColor(R, G, B)
}

// initSkyStars 初始化天空中的星星位置
func initSkyStars(width, topY int) {
	if topY <= TreeTopMargin {
		skyStars = nil
		return
	}
	skyStars = make([]SkyStar, 0, SkyStarCount)
	for i := 0; i < SkyStarCount; i++ {
		x := rnd.Intn(width)
		// 分布在 TreeTopMargin .. topY-1
		y := TreeTopMargin + rnd.Intn(max(1, topY-TreeTopMargin))
		phase := rnd.Float64() * 2 * math.Pi
		speed := 0.8 + rnd.Float64()*0.8
		skyStars = append(skyStars, SkyStar{X: x, Y: y, Phase: phase, Speed: speed})
	}
}

// 根据亮度生成天空背景颜色（基底加上亮度）
func skyBgColor(brightness float64) tcell.Color {
	br := int32(float64(SkyBaseR) + brightness*40.0)
	bg := int32(float64(SkyBaseG) + brightness*50.0)
	bb := int32(float64(SkyBaseB) + brightness*80.0)
	// 限制到 0..255
	if br > 255 { br = 255 }
	if bg > 255 { bg = 255 }
	if bb > 255 { bb = 255 }
	return tcell.NewRGBColor(br, bg, bb)
}

// 小工具：max（若已有项目中无此函数）
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}