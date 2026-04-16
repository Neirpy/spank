// spank detects slaps/hits on the laptop and plays audio responses.
// It reads the Apple Silicon accelerometer directly via IOKit HID —
// no separate sensor daemon required. Needs sudo.
package main

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"path/filepath"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/fang"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/spf13/cobra"
	"github.com/taigrr/apple-silicon-accelerometer/detector"
	"github.com/taigrr/apple-silicon-accelerometer/sensor"
	"github.com/taigrr/apple-silicon-accelerometer/shm"
)

var version = "dev"

//go:embed audio/pain/*.mp3
var painAudio embed.FS

//go:embed audio/sexy/*.mp3
var sexyAudio embed.FS

//go:embed audio/halo/*.mp3
var haloAudio embed.FS

//go:embed audio/donkeykong/*.mp3
var donkeyAudio embed.FS

//go:embed audio/sciencesco/*.mp3
var SCAudio embed.FS

var (
	painMode      bool
	donkeyMode    bool
	sexyMode      bool
	haloMode      bool
	SCMode        bool
	customPath    string
	customFiles   []string
	fastMode      bool
	minAmplitude  float64
	cooldownMs    int
	stdioMode     bool
	volumeScaling bool
	paused        bool
	pausedMu      sync.RWMutex
	speedRatio    float64
	uiMode        bool
	activeCmd     *exec.Cmd
	activeCmdMu   sync.Mutex
	activePackId  string
)

const uiHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Spank Control Center</title>
    <style>
        :root {
            --bg-color: #0f172a;
            --text-color: #f8fafc;
            --accent: #3b82f6;
            --accent-hover: #2563eb;
            --glass-bg: rgba(255, 255, 255, 0.05);
            --glass-border: rgba(255, 255, 255, 0.1);
            --success: #10b981;
            --danger: #ef4444;
            --card-bg: rgba(30, 41, 59, 0.7);
        }
        body {
            font-family: 'Inter', system-ui, -apple-system, sans-serif;
            background: radial-gradient(circle at 50% -20%, #1e293b, #0f172a);
            color: var(--text-color);
            min-height: 100vh;
            margin: 0;
            padding: 2rem;
            box-sizing: border-box;
        }
        .header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 3rem;
            background: var(--glass-bg);
            backdrop-filter: blur(10px);
            padding: 1rem 2rem;
            border-radius: 16px;
            border: 1px solid var(--glass-border);
        }
        h1 {
            margin: 0;
            font-size: 2rem;
            font-weight: 800;
            background: linear-gradient(135deg, #60a5fa, #a78bfa);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .btn {
            background: var(--accent);
            color: white;
            border: none;
            padding: 0.75rem 1.5rem;
            border-radius: 9999px;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
        }
        .btn:hover { background: var(--accent-hover); transform: translateY(-2px); }
        .btn-danger { background: rgba(239, 68, 68, 0.2); color: #fca5a5; border: 1px solid var(--danger); }
        .btn-danger:hover { background: var(--danger); color: white; }
        .btn-stop { background: var(--danger); color: white; }
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
            gap: 1.5rem;
        }
        .card {
            background: var(--card-bg);
            border: 1px solid var(--glass-border);
            border-radius: 16px;
            padding: 1.5rem;
            backdrop-filter: blur(10px);
            transition: all 0.3s ease;
            display: flex;
            flex-direction: column;
            justify-content: space-between;
        }
        .card:hover { transform: translateY(-5px); border-color: var(--accent); }
        .card h3 { margin-top: 0; font-size: 1.25rem; font-weight: 600; }
        .card-actions { display: flex; gap: 0.5rem; margin-top: 1rem; }
        .card-actions button { flex: 1; }
        .playing-indicator {
            color: var(--success);
            font-size: 0.8rem;
            text-transform: uppercase;
            letter-spacing: 1px;
            font-weight: bold;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        .playing-indicator::before {
            content: '';
            display: block;
            width: 8px;
            height: 8px;
            background: var(--success);
            border-radius: 50%;
            animation: pulse 1.5s infinite;
        }
        @keyframes pulse {
            0% { box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7); }
            70% { box-shadow: 0 0 0 10px rgba(16, 185, 129, 0); }
            100% { box-shadow: 0 0 0 0 rgba(16, 185, 129, 0); }
        }
        .upload-zone {
            border: 2px dashed #475569;
            border-radius: 16px;
            padding: 2rem;
            text-align: center;
            cursor: pointer;
            transition: all 0.3s;
            margin-top: 3rem;
            background: rgba(15, 23, 42, 0.4);
        }
        .upload-zone.dragover { border-color: var(--accent); background: rgba(59, 130, 246, 0.1); }
        #pack-name {
            background: rgba(0,0,0,0.3);
            border: 1px solid var(--glass-border);
            color: white;
            padding: 0.75rem 1rem;
            border-radius: 8px;
            margin-bottom: 1rem;
            width: 100%;
            max-width: 300px;
            font-size: 1rem;
        }
        .status { margin-top: 1rem; padding: 1rem; border-radius: 8px; display: none; }
        .success { background: rgba(16, 185, 129, 0.1); color: var(--success); }
        .error { background: rgba(239, 68, 68, 0.1); color: var(--danger); }
        .global-stop-btn {
            background: var(--danger);
            color: white;
            font-size: 1.25rem;
            padding: 1rem 2rem;
            margin-bottom: 2rem;
            width: 100%;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🍑 Spank Control Center</h1>
    </div>

    <button id="globalStopBtn" class="btn global-stop-btn" style="display: none;" onclick="stopPack()">🛑 STOP ACTIVE SLAP LISTENER</button>

    <h2>Available Packs</h2>
    <div class="grid" id="packs-grid">
        <!-- populated by JS -->
    </div>

    <div class="upload-zone" id="drop-zone">
        <h3>🎨 Create New Custom Pack</h3>
        <input type="text" id="pack-name" placeholder="Name your pack (e.g., My Sounds)" onclick="event.stopPropagation()">
        <p>Type a name above, then Drag & Drop your audio files here.</p>
        <input type="file" id="file-input" multiple accept=".mp3" style="display: none;">
        <div id="upload-status" class="status"></div>
    </div>

<script>
    let activePack = null;

    async function loadPacks() {
        try {
            const res = await fetch('/api/packs');
            const data = await res.json();
            activePack = data.active;
            const grid = document.getElementById('packs-grid');
            grid.innerHTML = '';

            if (activePack) {
                document.getElementById('globalStopBtn').style.display = 'block';
            } else {
                document.getElementById('globalStopBtn').style.display = 'none';
            }

            data.packs.forEach(pack => {
                const isPlaying = activePack && activePack.id === pack.id;
                const card = document.createElement('div');
                card.className = 'card';
                
                let deleteBtn = '';
                if (pack.type === 'custom') {
                    deleteBtn = '<button class="btn btn-danger" onclick="deletePack(\'' + pack.id + '\')">🗑️ Delete</button>';
                }

                let playBtn = isPlaying 
                    ? '<button class="btn btn-stop" onclick="stopPack()">🛑 Stop</button>'
                    : '<button class="btn" onclick="playPack(\''+pack.type+'\', \''+pack.id+'\')">▶️ Play</button>';

                let countText = pack.type === 'custom' ? ('(' + pack.count + ' sounds)') : 'Built-in pack';

                card.innerHTML = 
                    '<div>' +
                        '<h3>' + pack.name + '</h3>' +
                        '<p style="color: #94a3b8; font-size: 0.9rem;">' + countText + '</p>' +
                        (isPlaying ? '<div class="playing-indicator">LISTENING FOR SLAPS...</div>' : '') +
                    '</div>' +
                    '<div class="card-actions">' +
                        playBtn +
                        deleteBtn +
                    '</div>';
                grid.appendChild(card);
            });
        } catch(e) { }
    }

    async function playPack(type, id) {
        await fetch('/api/play', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({type, id})
        });
        loadPacks();
    }

    async function stopPack() {
        await fetch('/api/stop', { method: 'POST' });
        loadPacks();
    }

    async function deletePack(id) {
        if(!confirm('Are you sure you want to delete this custom pack?')) return;
        await fetch('/api/packs?id=' + encodeURIComponent(id), { method: 'DELETE' });
        loadPacks();
    }


    const dropZone = document.getElementById('drop-zone');
    const fileInput = document.getElementById('file-input');
    const statusEl = document.getElementById('upload-status');
    const packNameInput = document.getElementById('pack-name');

    ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(e => {
        dropZone.addEventListener(e, preventDefaults, false);
        document.body.addEventListener(e, preventDefaults, false);
    });

    function preventDefaults(e) { e.preventDefault(); e.stopPropagation(); }

    ['dragenter', 'dragover'].forEach(e => dropZone.addEventListener(e, () => dropZone.classList.add('dragover'), false));
    ['dragleave', 'drop'].forEach(e => dropZone.addEventListener(e, () => dropZone.classList.remove('dragover'), false));

    dropZone.addEventListener('drop', handleDrop, false);
    dropZone.addEventListener('click', () => { fileInput.click(); });
    fileInput.addEventListener('change', function() { handleFiles(Array.from(this.files)); });

    async function handleDrop(e) {
        let files = [];
        if (e.dataTransfer.items) {
            const items = Array.from(e.dataTransfer.items).map(item => item.webkitGetAsEntry());
            files = await getFilesFromEntries(items);
        } else {
            files = Array.from(e.dataTransfer.files);
        }
        handleFiles(files);
    }
    
    async function getFilesFromEntries(entries) {
        let files = [];
        for (let entry of entries) {
            if (entry) {
                if (entry.isFile) {
                    try {
                        const file = await new Promise((resolve, reject) => entry.file(resolve, reject));
                        files.push(file);
                    } catch (err) { }
                } else if (entry.isDirectory) {
                    const dirReader = entry.createReader();
                    const entriesInDir = await new Promise((resolve, reject) => {
                        let ents = [];
                        function read() {
                            dirReader.readEntries(res => {
                                if(!res.length) resolve(ents);
                                else { ents = ents.concat(Array.from(res)); read(); }
                            });
                        }
                        read();
                    });
                    const subFiles = await getFilesFromEntries(entriesInDir);
                    files = files.concat(subFiles);
                }
            }
        }
        return files;
    }

    function handleFiles(files) {
        let name = packNameInput.value.trim();
        if (!name) {
            showStatus('Please enter a pack name before dropping files!', 'error');
            return;
        }
        // Only accept alphanumeric and basic characters securely.
        if (!/^[a-zA-Z0-9 _-]+$/.test(name)) {
            showStatus('Invalid pack name (use letters, numbers, spaces, -, _)', 'error');
            return;
        }

        let mp3s = files.filter(f => f.name.toLowerCase().endsWith('.mp3'));
        if (mp3s.length === 0) {
            showStatus('No MP3 files found!', 'error');
            return;
        }
        mp3s.sort((a, b) => a.name.localeCompare(b.name, undefined, {numeric: true, sensitivity: 'base'}));
        
        showStatus('Uploading ' + mp3s.length + ' files...', 'success');
        const formData = new FormData();
        formData.append('packName', name);
        mp3s.forEach(f => formData.append('files', f));

        fetch('/api/upload', { method: 'POST', body: formData })
        .then(res => res.json())
        .then(data => {
            if (data.success) {
                showStatus('✅ Pack created successfully!', 'success');
                packNameInput.value = '';
                loadPacks();
            } else showStatus(data.error, 'error');
        }).catch(err => showStatus('Error: '+err, 'error'));
    }

    function showStatus(msg, type) {
        statusEl.style.display = 'block';
        statusEl.className = 'status ' + type;
        statusEl.innerHTML = msg;
        setTimeout(() => { statusEl.style.display = 'none'; }, 5000);
    }

    setInterval(loadPacks, 2000);
    loadPacks();
</script>
</body>
</html>
`

// sensorReady is closed once shared memory is created and the sensor
// worker is about to enter the CFRunLoop.
var sensorReady = make(chan struct{})

// sensorErr receives any error from the sensor worker.
var sensorErr = make(chan error, 1)

type playMode int

const (
	modeRandom playMode = iota
	modeEscalation
)

const (
	// decayHalfLife is how many seconds of inactivity before intensity
	// halves. Controls how fast escalation fades.
	decayHalfLife = 30.0

	// defaultMinAmplitude is the default detection threshold.
	defaultMinAmplitude = 0.05

	// defaultCooldownMs is the default cooldown between audio responses.
	defaultCooldownMs = 750

	// defaultSpeedRatio is the default playback speed (1.0 = normal).
	defaultSpeedRatio = 1.0

	// defaultSensorPollInterval is how often we check for new accelerometer data.
	defaultSensorPollInterval = 10 * time.Millisecond

	// defaultMaxSampleBatch caps the number of accelerometer samples processed
	// per tick to avoid falling behind.
	defaultMaxSampleBatch = 200

	// sensorStartupDelay gives the sensor time to start producing data.
	sensorStartupDelay = 100 * time.Millisecond
)

type runtimeTuning struct {
	minAmplitude float64
	cooldown     time.Duration
	pollInterval time.Duration
	maxBatch     int
}

func defaultTuning() runtimeTuning {
	return runtimeTuning{
		minAmplitude: defaultMinAmplitude,
		cooldown:     time.Duration(defaultCooldownMs) * time.Millisecond,
		pollInterval: defaultSensorPollInterval,
		maxBatch:     defaultMaxSampleBatch,
	}
}

func applyFastOverlay(base runtimeTuning) runtimeTuning {
	base.pollInterval = 4 * time.Millisecond
	base.cooldown = 350 * time.Millisecond
	if base.minAmplitude > 0.18 {
		base.minAmplitude = 0.18
	}
	if base.maxBatch < 320 {
		base.maxBatch = 320
	}
	return base
}

type soundPack struct {
	name   string
	fs     embed.FS
	dir    string
	mode   playMode
	files  []string
	custom bool
}

func (sp *soundPack) loadFiles() error {
	if sp.custom {
		entries, err := os.ReadDir(sp.dir)
		if err != nil {
			return err
		}
		sp.files = make([]string, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				sp.files = append(sp.files, sp.dir+"/"+entry.Name())
			}
		}
	} else {
		entries, err := sp.fs.ReadDir(sp.dir)
		if err != nil {
			return err
		}
		sp.files = make([]string, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				sp.files = append(sp.files, sp.dir+"/"+entry.Name())
			}
		}
	}
	sort.Strings(sp.files)
	if len(sp.files) == 0 {
		return fmt.Errorf("no audio files found in %s", sp.dir)
	}
	return nil
}

type slapTracker struct {
	mu       sync.Mutex
	score    float64
	lastTime time.Time
	total    int
	halfLife float64 // seconds
	scale    float64 // controls the escalation curve shape
	pack     *soundPack
}

func newSlapTracker(pack *soundPack, cooldown time.Duration) *slapTracker {
	// scale maps the exponential curve so that sustained max-rate
	// slapping (one per cooldown) reaches the final file. At steady
	// state the score converges to ssMax; we set scale so that score
	// maps to the last index.
	cooldownSec := cooldown.Seconds()
	ssMax := 1.0 / (1.0 - math.Pow(0.5, cooldownSec/decayHalfLife))
	scale := (ssMax - 1) / math.Log(float64(len(pack.files)+1))
	return &slapTracker{
		halfLife: decayHalfLife,
		scale:    scale,
		pack:     pack,
	}
}

func (st *slapTracker) record(now time.Time) (int, float64) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if !st.lastTime.IsZero() {
		elapsed := now.Sub(st.lastTime).Seconds()
		st.score *= math.Pow(0.5, elapsed/st.halfLife)
	}
	st.score += 1.0
	st.lastTime = now
	st.total++
	return st.total, st.score
}

func (st *slapTracker) getFile(score float64) string {
	if st.pack.mode == modeRandom {
		return st.pack.files[rand.Intn(len(st.pack.files))]
	}

	// Escalation: 1-exp(-x) curve maps score to file index.
	// At sustained max slap rate, score reaches ssMax which maps
	// to the final file.
	maxIdx := len(st.pack.files) - 1
	idx := min(int(float64(len(st.pack.files))*(1.0-math.Exp(-(score-1)/st.scale))), maxIdx)
	return st.pack.files[idx]
}

func main() {
	cmd := &cobra.Command{
		Use:   "spank",
		Short: "Yells 'ow!' when you slap the laptop",
		Long: `spank reads the Apple Silicon accelerometer directly via IOKit HID
and plays audio responses when a slap or hit is detected.

Requires sudo (for IOKit HID access to the accelerometer).

Use --sexy for a different experience. In sexy mode, the more you slap
within a minute, the more intense the sounds become.

Use --halo to play random audio clips from Halo soundtracks on each slap.`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			tuning := defaultTuning()
			if fastMode {
				tuning = applyFastOverlay(tuning)
			}
			// Explicit flags override fast preset defaults
			if cmd.Flags().Changed("min-amplitude") {
				tuning.minAmplitude = minAmplitude
			}
			if cmd.Flags().Changed("cooldown") {
				tuning.cooldown = time.Duration(cooldownMs) * time.Millisecond
			}
			return run(cmd.Context(), tuning)
		},
		SilenceUsage: true,
	}

	cmd.Flags().BoolVarP(&uiMode, "ui", "u", false, "Start the Web UI to create a custom spank pack")
	cmd.Flags().BoolVarP(&sexyMode, "sexy", "s", false, "Enable sexy mode")
	cmd.Flags().BoolVarP(&haloMode, "halo", "H", false, "Enable halo mode")
	cmd.Flags().BoolVarP(&donkeyMode, "donkey", "d", false, "Enable donkey mode")
	cmd.Flags().BoolVarP(&painMode, "pain", "p", false, "Enable pain mode")
	cmd.Flags().BoolVarP(&SCMode, "SC", "C", false, "Enable SCo mode")
	cmd.Flags().StringVarP(&customPath, "custom", "c", "", "Path to custom MP3 audio directory")
	cmd.Flags().BoolVar(&fastMode, "fast", false, "Enable faster detection tuning (shorter cooldown, higher sensitivity)")
	cmd.Flags().StringSliceVar(&customFiles, "custom-files", nil, "Comma-separated list of custom MP3 files")
	cmd.Flags().Float64Var(&minAmplitude, "min-amplitude", defaultMinAmplitude, "Minimum amplitude threshold (0.0-1.0, lower = more sensitive)")
	cmd.Flags().IntVar(&cooldownMs, "cooldown", defaultCooldownMs, "Cooldown between responses in milliseconds")
	cmd.Flags().BoolVar(&stdioMode, "stdio", false, "Enable stdio mode: JSON output and stdin commands (for GUI integration)")
	cmd.Flags().BoolVar(&volumeScaling, "volume-scaling", false, "Scale playback volume by slap amplitude (harder hits = louder)")
	cmd.Flags().Float64Var(&speedRatio, "speed", defaultSpeedRatio, "Playback speed multiplier (0.5 = half speed, 2.0 = double speed)")

	if err := fang.Execute(context.Background(), cmd); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, tuning runtimeTuning) error {
	if uiMode {
		return runUI()
	}

	if os.Geteuid() != 0 {
		return fmt.Errorf("spank requires root privileges for accelerometer access, run with: sudo spank")
	}

	modeCount := 0
	if sexyMode {
		modeCount++
	}
	if haloMode {
		modeCount++
	}
	if donkeyMode {
		modeCount++
	}
	if painMode {
		modeCount++
	}

	if SCMode {
		modeCount++
	}

	if customPath != "" || len(customFiles) > 0 {
		modeCount++
	}
	if modeCount > 1 {
		return fmt.Errorf("--sexy, --halo, --donkey, --pain, --SC, and --custom/--custom-files are mutually exclusive; pick one")
	}

	if tuning.minAmplitude < 0 || tuning.minAmplitude > 1 {
		return fmt.Errorf("--min-amplitude must be between 0.0 and 1.0")
	}
	if tuning.cooldown <= 0 {
		return fmt.Errorf("--cooldown must be greater than 0")
	}

	var pack *soundPack
	switch {
	case len(customFiles) > 0:
		// Validate all files exist and are MP3s
		for _, f := range customFiles {
			if !strings.HasSuffix(strings.ToLower(f), ".mp3") {
				return fmt.Errorf("custom file must be MP3: %s", f)
			}
			if _, err := os.Stat(f); err != nil {
				return fmt.Errorf("custom file not found: %s", f)
			}
		}
		pack = &soundPack{name: "custom", mode: modeRandom, custom: true, files: customFiles}
	case customPath != "":
		pack = &soundPack{name: "custom", dir: customPath, mode: modeRandom, custom: true}
	case sexyMode:
		pack = &soundPack{name: "sexy", fs: sexyAudio, dir: "audio/sexy", mode: modeEscalation}
	case haloMode:
		pack = &soundPack{name: "halo", fs: haloAudio, dir: "audio/halo", mode: modeRandom}
	case painMode:
		pack = &soundPack{name: "pain", fs: painAudio, dir: "audio/pain", mode: modeRandom}
	case SCMode:
		pack = &soundPack{name: "SC", fs: SCAudio, dir: "audio/sciencesco", mode: modeRandom}
	default:
		pack = &soundPack{name: "donkey", fs: donkeyAudio, dir: "audio/donkeykong", mode: modeRandom}
	}

	// Only load files if not already set (customFiles case)
	if len(pack.files) == 0 {
		if err := pack.loadFiles(); err != nil {
			return fmt.Errorf("loading %s audio: %w", pack.name, err)
		}
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create shared memory for accelerometer data.
	accelRing, err := shm.CreateRing(shm.NameAccel)
	if err != nil {
		return fmt.Errorf("creating accel shm: %w", err)
	}
	defer accelRing.Close()
	defer accelRing.Unlink()

	// Start the sensor worker in a background goroutine.
	// sensor.Run() needs runtime.LockOSThread for CFRunLoop, which it
	// handles internally. We launch detection on the current goroutine.
	go func() {
		close(sensorReady)
		if err := sensor.Run(sensor.Config{
			AccelRing: accelRing,
			Restarts:  0,
		}); err != nil {
			sensorErr <- err
		}
	}()

	// Wait for sensor to be ready.
	select {
	case <-sensorReady:
	case err := <-sensorErr:
		return fmt.Errorf("sensor worker failed: %w", err)
	case <-ctx.Done():
		return nil
	}

	// Give the sensor a moment to start producing data.
	time.Sleep(sensorStartupDelay)

	return listenForSlaps(ctx, pack, accelRing, tuning)
}

func listenForSlaps(ctx context.Context, pack *soundPack, accelRing *shm.RingBuffer, tuning runtimeTuning) error {
	tracker := newSlapTracker(pack, tuning.cooldown)
	speakerInit := false
	det := detector.New()
	var lastAccelTotal uint64
	var lastEventTime time.Time
	var lastYell time.Time

	// Start stdin command reader if in JSON mode
	if stdioMode {
		go readStdinCommands()
	}

	presetLabel := "default"
	if fastMode {
		presetLabel = "fast"
	}
	fmt.Printf("spank: listening for slaps in %s mode with %s tuning... (ctrl+c to quit)\n", pack.name, presetLabel)
	if stdioMode {
		fmt.Println(`{"status":"ready"}`)
	}

	ticker := time.NewTicker(tuning.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nbye!")
			return nil
		case err := <-sensorErr:
			return fmt.Errorf("sensor worker failed: %w", err)
		case <-ticker.C:
		}

		// Check if paused
		pausedMu.RLock()
		isPaused := paused
		pausedMu.RUnlock()
		if isPaused {
			continue
		}

		now := time.Now()
		tNow := float64(now.UnixNano()) / 1e9

		samples, newTotal := accelRing.ReadNew(lastAccelTotal, shm.AccelScale)
		lastAccelTotal = newTotal
		if len(samples) > tuning.maxBatch {
			samples = samples[len(samples)-tuning.maxBatch:]
		}

		nSamples := len(samples)
		for idx, sample := range samples {
			tSample := tNow - float64(nSamples-idx-1)/float64(det.FS)
			det.Process(sample.X, sample.Y, sample.Z, tSample)
		}

		if len(det.Events) == 0 {
			continue
		}

		ev := det.Events[len(det.Events)-1]
		if ev.Time.Equal(lastEventTime) {
			continue
		}
		lastEventTime = ev.Time

		if time.Since(lastYell) <= time.Duration(cooldownMs)*time.Millisecond {
			continue
		}
		if ev.Amplitude < minAmplitude {
			continue
		}

		lastYell = now
		num, score := tracker.record(now)
		file := tracker.getFile(score)
		if stdioMode {
			event := map[string]interface{}{
				"timestamp":  now.Format(time.RFC3339Nano),
				"slapNumber": num,
				"amplitude":  ev.Amplitude,
				"severity":   string(ev.Severity),
				"file":       file,
			}
			if data, err := json.Marshal(event); err == nil {
				fmt.Println(string(data))
			}
		} else {
			fmt.Printf("slap #%d [%s amp=%.5fg] -> %s\n", num, ev.Severity, ev.Amplitude, file)
		}
		go playAudio(pack, file, ev.Amplitude, &speakerInit)
	}
}

var speakerMu sync.Mutex

// amplitudeToVolume maps a detected amplitude to a beep/effects.Volume
// level. Amplitude typically ranges from ~0.05 (light tap) to ~1.0+
// (hard slap). The mapping uses a logarithmic curve so that light taps
// are noticeably quieter and hard hits play near full volume.
//
// Returns a value in the range [-3.0, 0.0] for use with effects.Volume
// (base 2): -3.0 is ~1/8 volume, 0.0 is full volume.
func amplitudeToVolume(amplitude float64) float64 {
	const (
		minAmp = 0.05 // softest detectable
		maxAmp = 0.80 // treat anything above this as max
		minVol = -3.0 // quietest playback (1/8 volume with base 2)
		maxVol = 0.0  // full volume
	)

	// Clamp
	if amplitude <= minAmp {
		return minVol
	}
	if amplitude >= maxAmp {
		return maxVol
	}

	// Normalize to [0, 1]
	t := (amplitude - minAmp) / (maxAmp - minAmp)

	// Log curve for more natural volume scaling
	// log(1 + t*99) / log(100) maps [0,1] -> [0,1] with a log curve
	t = math.Log(1+t*99) / math.Log(100)

	return minVol + t*(maxVol-minVol)
}

func playAudio(pack *soundPack, path string, amplitude float64, speakerInit *bool) {
	var streamer beep.StreamSeekCloser
	var format beep.Format

	if pack.custom {
		file, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "spank: open %s: %v\n", path, err)
			return
		}
		defer file.Close()
		streamer, format, err = mp3.Decode(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "spank: decode %s: %v\n", path, err)
			return
		}
	} else {
		data, err := pack.fs.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "spank: read %s: %v\n", path, err)
			return
		}
		streamer, format, err = mp3.Decode(io.NopCloser(bytes.NewReader(data)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "spank: decode %s: %v\n", path, err)
			return
		}
	}
	defer streamer.Close()

	speakerMu.Lock()
	if !*speakerInit {
		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		*speakerInit = true
	}
	speakerMu.Unlock()

	// Optionally scale volume based on slap amplitude
	var source beep.Streamer = streamer
	if volumeScaling {
		source = &effects.Volume{
			Streamer: streamer,
			Base:     2,
			Volume:   amplitudeToVolume(amplitude),
			Silent:   false,
		}
	}

	// Apply speed change via resampling trick:
	// Claiming the audio is at rate*speed and resampling back to rate
	// makes the speaker consume samples faster/slower.
	if speedRatio != 1.0 && speedRatio > 0 {
		fakeRate := beep.SampleRate(int(float64(format.SampleRate) * speedRatio))
		source = beep.Resample(4, fakeRate, format.SampleRate, source)
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(source, beep.Callback(func() {
		done <- true
	})))
	<-done
}

// stdinCommand represents a command received via stdin
type stdinCommand struct {
	Cmd       string  `json:"cmd"`
	Amplitude float64 `json:"amplitude,omitempty"`
	Cooldown  int     `json:"cooldown,omitempty"`
	Speed     float64 `json:"speed,omitempty"`
}

// readStdinCommands reads JSON commands from stdin for live control
func readStdinCommands() {
	processCommands(os.Stdin, os.Stdout)
}

// processCommands reads JSON commands from r and writes responses to w.
// This is the testable core of the stdin command handler.
func processCommands(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var cmd stdinCommand
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			if stdioMode {
				fmt.Fprintf(w, `{"error":"invalid command: %s"}%s`, err.Error(), "\n")
			}
			continue
		}

		switch cmd.Cmd {
		case "pause":
			pausedMu.Lock()
			paused = true
			pausedMu.Unlock()
			if stdioMode {
				fmt.Fprintln(w, `{"status":"paused"}`)
			}
		case "resume":
			pausedMu.Lock()
			paused = false
			pausedMu.Unlock()
			if stdioMode {
				fmt.Fprintln(w, `{"status":"resumed"}`)
			}
		case "set":
			if cmd.Amplitude > 0 && cmd.Amplitude <= 1 {
				minAmplitude = cmd.Amplitude
			}
			if cmd.Cooldown > 0 {
				cooldownMs = cmd.Cooldown
			}
			if cmd.Speed > 0 {
				speedRatio = cmd.Speed
			}
			if stdioMode {
				fmt.Fprintf(w, `{"status":"settings_updated","amplitude":%.4f,"cooldown":%d,"speed":%.2f}%s`, minAmplitude, cooldownMs, speedRatio, "\n")
			}
		case "volume-scaling":
			volumeScaling = !volumeScaling
			if stdioMode {
				fmt.Fprintf(w, `{"status":"volume_scaling_toggled","volume_scaling":%t}%s`, volumeScaling, "\n")
			}
		case "status":
			pausedMu.RLock()
			isPaused := paused
			pausedMu.RUnlock()
			if stdioMode {
				fmt.Fprintf(w, `{"status":"ok","paused":%t,"amplitude":%.4f,"cooldown":%d,"volume_scaling":%t,"speed":%.2f}%s`, isPaused, minAmplitude, cooldownMs, volumeScaling, speedRatio, "\n")
			}
		default:
			if stdioMode {
				fmt.Fprintf(w, `{"error":"unknown command: %s"}%s`, cmd.Cmd, "\n")
			}
		}
	}
}

func runUI() error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(uiHTML))
	})

	http.HandleFunc("/api/packs", handlePacks)
	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/api/play", handlePlay)
	http.HandleFunc("/api/stop", handleStop)
	http.HandleFunc("/api/quit", handleQuit)

	port := 8080
	url := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("spank: starting Web UI at %s\n", url)

	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

func getPacks() []map[string]interface{} {
	packs := []map[string]interface{}{
		{"id": "donkey", "name": "Donkey (Default)", "type": "default"},
		{"id": "sexy", "name": "Sexy (Escalation)", "type": "default"},
		{"id": "halo", "name": "Halo", "type": "default"},
		{"id": "pain", "name": "Pain", "type": "default"},
		{"id": "SC", "name": "SCo", "type": "default"},
	}

	customDir := "/Users/Shared/SpankPacks"
	entries, err := os.ReadDir(customDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				// Count mp3s
				count := 0
				mp3s, _ := os.ReadDir(filepath.Join(customDir, e.Name()))
				for _, f := range mp3s {
					if strings.HasSuffix(strings.ToLower(f.Name()), ".mp3") {
						count++
					}
				}
				packs = append(packs, map[string]interface{}{
					"id":    e.Name(),
					"name":  e.Name(),
					"type":  "custom",
					"count": count,
				})
			}
		}
	}
	return packs
}

func handlePacks(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		id := r.URL.Query().Get("id")
		if id != "" && !strings.Contains(id, "..") && !strings.Contains(id, "/") {
			os.RemoveAll(filepath.Join("/Users/Shared/SpankPacks", id))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	activeCmdMu.Lock()
	var act map[string]string
	// Only consider it active if the process is actually running
	if activeCmd != nil && activeCmd.Process != nil && activePackId != "" {
		act = map[string]string{"id": activePackId}
	}
	activeCmdMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"packs":  getPacks(),
		"active": act,
	})
}

func handlePlay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"`
		Id   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body")
		return
	}

	activeCmdMu.Lock()
	defer activeCmdMu.Unlock()

	if activeCmd != nil && activeCmd.Process != nil {
		activeCmd.Process.Kill()
		activeCmd.Wait()
		activeCmd = nil
		activePackId = ""
	}

	args := []string{}
	// Important: We must not run uiMode recursively!
	if req.Type == "custom" {
		if strings.Contains(req.Id, "..") || strings.Contains(req.Id, "/") {
			jsonError(w, "Invalid pack id")
			return
		}
		args = append(args, "--custom", filepath.Join("/Users/Shared/SpankPacks", req.Id))
	} else {
		args = append(args, "--"+req.Id)
	}

	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Start(); err != nil {
		jsonError(w, "Failed to start tracking")
		return
	}
	
	// Helper goroutine to clear activeCmd state when it stops inherently
	go func(c *exec.Cmd, packId string) {
		c.Wait()
		activeCmdMu.Lock()
		// Only clear if another process didn't take over
		if activeCmd == c {
			activeCmd = nil
			activePackId = ""
		}
		activeCmdMu.Unlock()
	}(cmd, req.Id)

	activeCmd = cmd
	activePackId = req.Id

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	activeCmdMu.Lock()
	defer activeCmdMu.Unlock()

	if activeCmd != nil && activeCmd.Process != nil {
		activeCmd.Process.Kill()
		activeCmd.Wait()
		activeCmd = nil
		activePackId = ""
	}
	w.WriteHeader(http.StatusOK)
}

func handleQuit(w http.ResponseWriter, r *http.Request) {
	go func() {
		activeCmdMu.Lock()
		if activeCmd != nil && activeCmd.Process != nil {
			activeCmd.Process.Kill()
		}
		activeCmdMu.Unlock()
		time.Sleep(200 * time.Millisecond)
		os.Exit(0)
	}()
	w.WriteHeader(http.StatusOK)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(100 << 20)
	if err != nil {
		jsonError(w, "File size too large or invalid request")
		return
	}

	packName := r.FormValue("packName")
	if packName == "" || strings.Contains(packName, "/") || strings.Contains(packName, "\\") || strings.Contains(packName, "..") {
		jsonError(w, "Invalid or missing packName")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		jsonError(w, "No files provided")
		return
	}

	outDir := filepath.Join("/Users/Shared/SpankPacks", packName)
	if err := os.MkdirAll(outDir, 0777); err != nil {
		jsonError(w, fmt.Sprintf("Could not create directory: %v", err))
		return
	}

	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			jsonError(w, fmt.Sprintf("Error opening file %s", fileHeader.Filename))
			return
		}

		outFileName := fmt.Sprintf("%02d.mp3", i)
		outFilePath := filepath.Join(outDir, outFileName)
		
		outFile, err := os.Create(outFilePath)
		if err != nil {
			file.Close()
			jsonError(w, fmt.Sprintf("Error creating %s", outFileName))
			return
		}

		_, err = io.Copy(outFile, file)
		outFile.Close()
		file.Close()

		if err != nil {
			jsonError(w, fmt.Sprintf("Error writing %s", outFileName))
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"path":    outDir,
	})
}

func jsonError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}
