//go:build cgo
// +build cgo

package pocsag

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"runtime"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

const (
	vertexShaderSource = `
		#version 410
		in vec3 vp;
		in vec2 vt;
		out vec2 texCoord;
		void main() {
			texCoord = vt;
			gl_Position = vec4(vp, 1.0);
		}
	` + "\x00"

	fragmentShaderSource = `
		#version 410
		uniform sampler2D waterfallTex;
		uniform float shift;
		uniform float scale;
		in vec2 texCoord;
		out vec4 frag_colour;

		float mag2col_base2_blue(float val) {
			if (val <= -2.75) return 0.0;
			if (val <= -1.75) return val + 2.75;
			if (val <= -0.75) return -(val + 0.75);
			if (val <= 0.0) return 0.0;
			if (val >= 1.0) return 1.0;
			return val;
		}

		vec3 mag2col(float a) {
			return vec3(clamp(a + 1.0, 0.0, 1.0), 
			            clamp(a, 0.0, 1.0), 
			            mag2col_base2_blue(a - 1.0));
		}

		void main() {
			float intensity = texture(waterfallTex, texCoord).r;
			// Scale intensity. In PySDR: mag = 10*log10(abs(fft)**2) -> typically -80 to 0.
			// The original shader expects 'intensity + shift' where shift brings it to a positive range,
			// then 'scale' stretches it.
			float logMag = 10.0 * log(intensity + 1e-12) / log(10.0);
			float a = (logMag + shift) * scale;
			frag_colour = vec4(mag2col(a), 1.0);
		}
	` + "\x00"

	lineVertexShaderSource = `
		#version 410
		in vec3 vp;
		void main() {
			gl_Position = vec4(vp, 1.0);
		}
	` + "\x00"

	lineFragmentShaderSource = `
		#version 410
		out vec4 frag_colour;
		void main() {
			frag_colour = vec4(1.0, 1.0, 1.0, 0.2); // Faint white grid
		}
	` + "\x00"
)

// WaterfallGL handles OpenGL rendering for the waterfall
type WaterfallGL struct {
	window      *glfw.Window
	program     uint32
	lineProgram uint32
	texture     uint32
	width       int // framebuffer width in pixels
	height      int // framebuffer height in pixels
	currentRow  int

	vao     uint32
	vbo     uint32
	gridVAO uint32
	gridVBO uint32

	// Display parameters
	Shift float32
	Scale float32
}

func NewWaterfallGL(width, height int, headless bool) (*WaterfallGL, error) {
	err := glfw.Init()
	if err != nil {
		return nil, err
	}

	glfw.WindowHint(glfw.Resizable, glfw.False)
	if headless {
		// Visible=False alone might not create a proper framebuffer on some OS.
		// Use a standard hidden window.
		glfw.WindowHint(glfw.Visible, glfw.False)
	} else {
		glfw.WindowHint(glfw.Visible, glfw.True)
	}
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	window, err := glfw.CreateWindow(width, height, "POCSAG Waterfall - OpenGL", nil, nil)
	if err != nil {
		return nil, err
	}
	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		return nil, err
	}

	// Query actual framebuffer pixel dimensions — may differ from window size
	// on HiDPI / DPI-scaled displays (e.g. 125% DPI on Windows gives a
	// framebuffer 1.25× larger than the requested screen-coordinate size).
	fbWidth, fbHeight := window.GetFramebufferSize()

	// Set viewport to match the full framebuffer
	gl.Viewport(0, 0, int32(fbWidth), int32(fbHeight))
	program, err := newProgram(vertexShaderSource, fragmentShaderSource)
	if err != nil {
		return nil, err
	}

	lineProgram, err := newProgram(lineVertexShaderSource, lineFragmentShaderSource)
	if err != nil {
		return nil, err
	}

	// Create texture sized to the ACTUAL framebuffer pixels
	var texture uint32
	gl.GenTextures(1, &texture)
	gl.BindTexture(gl.TEXTURE_2D, texture)

	// Pre-fill texture with our expected noise floor intensity (approx -83 dB)
	// This prevents the uninitialized rows from reading as -120 dB (which renders as black).
	bgNoise := make([]float32, fbWidth*fbHeight)
	for i := range bgNoise {
		bgNoise[i] = 5e-9
	}

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.R32F, int32(fbWidth), int32(fbHeight), 0, gl.RED, gl.FLOAT, gl.Ptr(bgNoise))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)

	// Set up full-screen quad VAO
	var vao, vbo uint32
	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)

	// Set up Grid VAO
	var gridVAO, gridVBO uint32
	gl.GenVertexArrays(1, &gridVAO)
	gl.GenBuffers(1, &gridVBO)

	wgl := &WaterfallGL{
		window:      window,
		program:     program,
		lineProgram: lineProgram,
		texture:     texture,
		width:       fbWidth,  // use actual framebuffer pixel width
		height:      fbHeight, // use actual framebuffer pixel height
		vao:         vao,
		vbo:         vbo,
		gridVAO:     gridVAO,
		gridVBO:     gridVBO,
		// dB math calibrated from MEASURED power distribution of synthetic FSK:
		//   Background (p50):  -84.75 dB  -> very dark blue (a ≈ -1.0)
		//   Signal peak:       -18.79 dB  -> white         (a = +2.0)
		//   We want p50 at a=-1.0 and peak at a=+2.0
		//   So: (-84.75 + Shift) * Scale = -1.0
		//       (-18.79 + Shift) * Scale = +2.0
		//   Subtracting: 65.96 * Scale = 3.0  -> Scale = 3.0/65.96 = 0.0455
		//   Shift: -1.0/0.0455 + 84.75 = -21.98 + 84.75 = 62.77 ≈ 63
		//   Verify: (-18.79+63)*0.0455 = 44.21*0.0455 = 2.01 (white) OK
		//   Verify: (-84.75+63)*0.0455 = -21.75*0.0455 = -0.99 (dark blue) OK
		Shift: 63.0,
		Scale: 0.0455,
	}

	return wgl, nil
}

// AddLine adds a new row of data to the waterfall (scrolling)
func (w *WaterfallGL) AddLine(data []float32) {
	if len(data) > w.width {
		data = data[:w.width]
	}

	gl.BindTexture(gl.TEXTURE_2D, w.texture)
	gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, int32(w.currentRow), int32(len(data)), 1, gl.RED, gl.FLOAT, gl.Ptr(data))

	w.currentRow = (w.currentRow + 1) % w.height
}

func (w *WaterfallGL) Render() {
	if w.window.ShouldClose() {
		return
	}

	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.UseProgram(w.program)

	// Set uniforms
	gl.Uniform1f(gl.GetUniformLocation(w.program, gl.Str("shift\x00")), w.Shift)
	gl.Uniform1f(gl.GetUniformLocation(w.program, gl.Str("scale\x00")), w.Scale)

	// Draw two quads to handle the circular buffer wrapping
	// We want the waterfall to flow DOWNARDS from the Top.
	// Newest data (V = topY) is drawn at the top of the screen (screenY = 0)
	topY := float32(w.currentRow) / float32(w.height)

	// Quad 1: Top of screen, holds newest data back to exactly V=0
	w.drawQuad(0, topY, topY, 0.0)

	// Quad 2: Bottom of screen, holds wrapped older data V=1.0 down to V=topY
	w.drawQuad(topY, 1.0-topY, 1.0, topY)

	// Draw grid on top
	w.drawGrid()

	w.window.SwapBuffers()
	glfw.PollEvents()
}

func (w *WaterfallGL) drawGrid() {
	gl.UseProgram(w.lineProgram)
	gl.BindVertexArray(w.gridVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, w.gridVBO)

	var lines []float32
	// Vertical lines (frequency axis)
	for x := -1.0; x <= 1.0; x += 0.4 {
		lines = append(lines, float32(x), -1, 0, float32(x), 1, 0)
	}
	// Horizontal lines (time axis)
	for y := -1.0; y <= 1.0; y += 0.25 {
		lines = append(lines, -1, float32(y), 0, 1, float32(y), 0)
	}

	gl.BufferData(gl.ARRAY_BUFFER, len(lines)*4, gl.Ptr(lines), gl.STREAM_DRAW)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 0, nil)
	gl.DrawArrays(gl.LINES, 0, int32(len(lines)/3))
}

func (w *WaterfallGL) drawQuad(screenY, height, vStart, vEnd float32) {
	// Full screen quad vertically shifted
	// Points: [X, Y, Z, U, V]
	vertices := []float32{
		-1, 1 - 2*screenY, 0, 0, vStart,
		-1, 1 - 2*(screenY+height), 0, 0, vEnd,
		1, 1 - 2*screenY, 0, 1, vStart,
		1, 1 - 2*(screenY+height), 0, 1, vEnd,
	}

	gl.BindVertexArray(w.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, w.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STREAM_DRAW)

	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 5*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 5*4, gl.PtrOffset(3*4))

	gl.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
}

func (w *WaterfallGL) Close() {
	glfw.Terminate()
}

func (w *WaterfallGL) ShouldClose() bool {
	return w.window.ShouldClose()
}

func (w *WaterfallGL) SaveToPNG(filename string) error {
	// Re-draw everything into the back buffer, then ReadPixels captures it.
	// We do NOT call SwapBuffers here — ReadPixels always reads the back buffer
	// in double-buffered contexts (which is where we just drew).
	gl.ReadBuffer(gl.BACK) // Explicitly read from the back buffer
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.UseProgram(w.program)
	gl.Uniform1f(gl.GetUniformLocation(w.program, gl.Str("shift\x00")), w.Shift)
	gl.Uniform1f(gl.GetUniformLocation(w.program, gl.Str("scale\x00")), w.Scale)

	topY := float32(w.currentRow) / float32(w.height)
	w.drawQuad(0, topY, topY, 0.0)
	w.drawQuad(topY, 1.0-topY, 1.0, topY)
	w.drawGrid()

	gl.Finish() // Wait for all GPU commands to complete

	width := w.width
	height := w.height

	// Read from the back buffer (where we just drew, before any swap)
	pixelData := make([]byte, width*height*4)
	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pixelData))

	// Create image
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Flip image vertically because OpenGL's origin is bottom-left, but image's is top-left
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// OpenGL pixel index
			glY := height - 1 - y
			glIdx := (glY*width + x) * 4

			// Image pixel index
			imgIdx := (y*width + x) * 4

			img.Pix[imgIdx+0] = pixelData[glIdx+0] // R
			img.Pix[imgIdx+1] = pixelData[glIdx+1] // G
			img.Pix[imgIdx+2] = pixelData[glIdx+2] // B
			img.Pix[imgIdx+3] = 255                // A (ignore OpenGL alpha which might be 0)
		}
	}

	// Save to file
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	return png.Encode(out, img)
}

func newProgram(vertexShaderSource, fragmentShaderSource string) (uint32, error) {
	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}

	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	program := gl.CreateProgram()

	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to link program: %v", log)
	}

	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile %v: %v", shaderType, log)
	}

	return shader, nil
}
