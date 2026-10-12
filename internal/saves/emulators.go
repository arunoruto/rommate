package saves

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Emulator handlers with Linux defaults for native, Flatpak and AppImage
// installs plus EmuDeck and RetroDECK layouts. Each reads the emulator's own
// config for a custom save location first.

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

func expandHome(p string) string {
	if p == "~" {
		return home()
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home(), p[2:])
	}
	return p
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// firstExisting returns the first path that exists, with symlinks resolved
// (EmuDeck and RetroDECK link save folders in both directions).
func firstExisting(paths ...string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if real, err := filepath.EvalSymlinks(expandHome(p)); err == nil {
			return real
		}
	}
	return ""
}

// configValue reads `key = value` from an INI, TOML or RetroArch config,
// optionally inside [section]. Quotes around the value are removed.
func configValue(path, section, key string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	current := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.Trim(line, "[]")
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != key || (section != "" && current != section) {
			continue
		}
		return strings.Trim(strings.TrimSpace(v), `"'`), true
	}
	return "", false
}

// qtPath reads a path from a Qt config (Azahar, Eden). Qt marks values the
// emulator ignores with `key\default=true`.
func qtPath(cfg, section, key string) string {
	if d, ok := configValue(cfg, section, key+`\default`); ok && d == "true" {
		return ""
	}
	v, _ := configValue(cfg, section, key)
	return v
}

func stem(path string) string {
	b := filepath.Base(path)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

func newest(paths []string) string {
	var best string
	var bestTime int64
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && (best == "" || st.ModTime().UnixNano() > bestTime) {
			best, bestTime = p, st.ModTime().UnixNano()
		}
	}
	return best
}

// ── RetroArch ────────────────────────────────────────────────────────────────

// retroArch handles <rom>.srm saves. Save dir: savefile_directory, plus the
// ROM folder name with sort_savefiles_by_content_enable and the core's
// library name with sort_savefiles_enable (default on).
type retroArch struct{ cfg string }

func newRetroArch() Handler {
	cfg := firstExisting(
		"~/.var/app/org.libretro.RetroArch/config/retroarch/retroarch.cfg",
		"~/.config/retroarch/retroarch.cfg",
		"~/.var/app/net.retrodeck.retrodeck/config/retroarch/retroarch.cfg",
	)
	if cfg == "" {
		return nil
	}
	return &retroArch{cfg: cfg}
}

func (r *retroArch) ID() string { return "retroarch" }

func (r *retroArch) flag(key string, def bool) bool {
	v, ok := configValue(r.cfg, "", key)
	if !ok {
		return def
	}
	return v == "true"
}

// base is the save root, or "" for "next to the ROM".
func (r *retroArch) base() string {
	if r.flag("savefiles_in_content_dir", false) {
		return ""
	}
	v, ok := configValue(r.cfg, "", "savefile_directory")
	switch {
	case !ok || v == "default":
		return filepath.Join(filepath.Dir(r.cfg), "saves")
	case v == "":
		return ""
	case strings.HasPrefix(v, ":"): // relative to the RetroArch directory
		return filepath.Join(filepath.Dir(r.cfg), strings.TrimPrefix(strings.TrimPrefix(v, ":"), "/"))
	}
	return expandHome(v)
}

func (r *retroArch) dirFor(g Game) string {
	b := r.base()
	if b == "" {
		return filepath.Dir(g.Path)
	}
	return b
}

func (r *retroArch) Find(g Game) (LocalSave, bool, error) {
	name := stem(g.Path) + ".srm"
	dir := r.dirFor(g)
	// Search the root and two levels of content/core folders.
	var found []string
	for _, pattern := range []string{name, "*/" + name, "*/*/" + name} {
		m, _ := filepath.Glob(filepath.Join(dir, pattern))
		found = append(found, m...)
	}
	if p := newest(found); p != "" {
		return LocalSave{Emulator: r.ID(), Path: p, FileName: name}, true, nil
	}
	return LocalSave{}, false, nil
}

// coreNames are RetroArch's library names of ES-DE's default cores, used to
// place a downloaded save when saves are sorted by core.
var coreNames = map[string]string{
	"nes": "Mesen", "famicom": "Mesen", "fds": "Mesen", "snes": "Snes9x", "sfam": "Snes9x",
	"gb": "Gambatte", "gbc": "Gambatte", "gba": "mGBA", "n64": "Mupen64Plus-Next",
	"nds": "melonDS DS", "ps": "Beetle PSX", "psx": "Beetle PSX", "genesis": "Genesis Plus GX",
	"sms": "Genesis Plus GX", "gamegear": "Genesis Plus GX", "segacd": "Genesis Plus GX",
	"sega32": "PicoDrive", "tg16": "Beetle PCE", "turbografx-cd": "Beetle PCE",
	"lynx": "Beetle Lynx", "virtualboy": "Beetle VB", "wonderswan": "Beetle WonderSwan",
	"wonderswan-color": "Beetle WonderSwan", "neo-geo-pocket": "Beetle NeoPop",
	"neo-geo-pocket-color": "Beetle NeoPop", "atari2600": "Stella", "atari7800": "ProSystem",
}

func (r *retroArch) Target(g Game) (LocalSave, error) {
	name := stem(g.Path) + ".srm"
	dir := r.dirFor(g)
	if r.base() != "" {
		if r.flag("sort_savefiles_by_content_enable", false) {
			dir = filepath.Join(dir, filepath.Base(filepath.Dir(g.Path)))
		}
		if r.flag("sort_savefiles_enable", true) {
			core := coreNames[g.Platform]
			if core == "" {
				return LocalSave{}, fmt.Errorf("unknown RetroArch core for %s; play the game once so a save exists", g.Platform)
			}
			dir = filepath.Join(dir, core)
		}
	}
	return LocalSave{Emulator: r.ID(), Path: filepath.Join(dir, name), FileName: name}, nil
}

// retroArchPlatforms are the RomM platforms whose RetroArch cores keep a
// <rom>.srm save.
var retroArchPlatforms = map[string]bool{}

func init() {
	for p := range coreNames {
		retroArchPlatforms[p] = true
	}
}

// ── melonDS ──────────────────────────────────────────────────────────────────

// melonDS keeps <rom>.sav next to the ROM, or in [Instance0] SaveFilePath.
type melonDS struct{ cfg string }

func newMelonDS() Handler {
	cfg := firstExisting(
		"~/.config/melonDS/melonDS.toml",
		"~/.var/app/net.kuribo64.melonDS/config/melonDS/melonDS.toml",
		"~/.var/app/net.retrodeck.retrodeck/config/melonDS/melonDS.toml",
	)
	if cfg == "" {
		return nil
	}
	return &melonDS{cfg: cfg}
}

func (m *melonDS) ID() string { return "melonds" }

func (m *melonDS) Target(g Game) (LocalSave, error) {
	dir := filepath.Dir(g.Path)
	if v, _ := configValue(m.cfg, "Instance0", "SaveFilePath"); v != "" {
		dir = expandHome(v)
	}
	name := stem(g.Path) + ".sav"
	return LocalSave{Emulator: m.ID(), Path: filepath.Join(dir, name), FileName: name}, nil
}

func (m *melonDS) Find(g Game) (LocalSave, bool, error) {
	s, err := m.Target(g)
	return s, err == nil && exists(s.Path), err
}

// ── Azahar (3DS) ─────────────────────────────────────────────────────────────

// azahar keeps a folder per title:
// sdmc/Nintendo 3DS/<id0>/<id1>/title/<high>/<low>/data (lowercase hex).
type azahar struct{ sdmc string }

func newAzahar() Handler {
	custom := ""
	for _, cfg := range []string{"~/.config/azahar-emu/qt-config.ini", "~/.var/app/org.azahar_emu.Azahar/config/azahar-emu/qt-config.ini"} {
		if v := qtPath(expandHome(cfg), "Data%20Storage", "sdmc_directory"); v != "" {
			custom = v
			break
		}
	}
	sdmc := firstExisting(
		custom,
		"~/.local/share/azahar-emu/sdmc",
		"~/.var/app/org.azahar_emu.Azahar/data/azahar-emu/sdmc",
		"~/Emulation/storage/azahar/sdmc",
		"~/retrodeck/saves/n3ds/azahar/sdmc",
		"~/.local/share/citra-emu/sdmc",
		"~/.local/share/lime3ds-emu/sdmc",
	)
	if sdmc == "" {
		return nil
	}
	return &azahar{sdmc: sdmc}
}

func (a *azahar) ID() string { return "azahar" }

func (a *azahar) titleDir(g Game) (string, uint64, error) {
	id, err := cachedID(g.Path, func() (uint64, error) { return N3DSTitleID(g.Path) })
	if err != nil {
		return "", 0, fmt.Errorf("reading the 3DS title ID: %w", err)
	}
	return filepath.Join("title", fmt.Sprintf("%08x", id>>32), fmt.Sprintf("%08x", id&0xffffffff), "data"), id, nil
}

func (a *azahar) Find(g Game) (LocalSave, bool, error) {
	rel, id, err := a.titleDir(g)
	if err != nil {
		return LocalSave{}, false, err
	}
	m, _ := filepath.Glob(filepath.Join(a.sdmc, "Nintendo 3DS", "*", "*", rel))
	if p := newest(m); p != "" {
		return LocalSave{Emulator: a.ID(), Path: p, IsDir: true, FileName: fmt.Sprintf("%016X.zip", id)}, true, nil
	}
	return LocalSave{}, false, nil
}

func (a *azahar) Target(g Game) (LocalSave, error) {
	rel, id, err := a.titleDir(g)
	if err != nil {
		return LocalSave{}, err
	}
	// Reuse the existing id0/id1 folders; Azahar uses 32 zeros for both.
	ids := filepath.Join(strings.Repeat("0", 32), strings.Repeat("0", 32))
	if m, _ := filepath.Glob(filepath.Join(a.sdmc, "Nintendo 3DS", "*", "*")); len(m) > 0 {
		sort.Strings(m)
		ids, _ = filepath.Rel(filepath.Join(a.sdmc, "Nintendo 3DS"), m[0])
	}
	return LocalSave{Emulator: a.ID(), Path: filepath.Join(a.sdmc, "Nintendo 3DS", ids, rel), IsDir: true,
		FileName: fmt.Sprintf("%016X.zip", id)}, nil
}

// ── PPSSPP ───────────────────────────────────────────────────────────────────

// ppsspp keeps PSP/SAVEDATA/<DISCID><suffix>/ folders, several per game.
type ppsspp struct{ savedata string }

func newPPSSPP() Handler {
	dir := firstExisting(
		"~/.var/app/org.ppsspp.PPSSPP/config/ppsspp/PSP/SAVEDATA",
		"~/.config/ppsspp/PSP/SAVEDATA",
		"~/retrodeck/saves/PSP/PPSSPP-SA",
		"~/Emulation/saves/ppsspp/saves",
	)
	if dir == "" {
		return nil
	}
	return &ppsspp{savedata: dir}
}

func (p *ppsspp) ID() string { return "ppsspp" }

func (p *ppsspp) Target(g Game) (LocalSave, error) {
	ext := strings.ToLower(filepath.Ext(g.Path))
	if ext != ".iso" && ext != ".cso" {
		return LocalSave{}, fmt.Errorf("cannot read the disc ID from %s files (only .iso and .cso)", ext)
	}
	id, err := cachedString(g.Path, func() (string, error) { return PSPDiscID(g.Path) })
	if err != nil {
		return LocalSave{}, fmt.Errorf("reading the PSP disc ID: %w", err)
	}
	return LocalSave{Emulator: p.ID(), Path: p.savedata, IsDir: true, Prefix: id, FileName: id + ".zip"}, nil
}

func (p *ppsspp) Find(g Game) (LocalSave, bool, error) {
	s, err := p.Target(g)
	if err != nil {
		return LocalSave{}, false, err
	}
	m, _ := filepath.Glob(filepath.Join(p.savedata, s.Prefix+"*"))
	return s, len(m) > 0, nil
}

// ── Eden and other yuzu forks (Switch) ───────────────────────────────────────

// yuzuFamily keeps nand/user/save/0000000000000000/<user>/<TITLEID>/
// (uppercase hex). Eden, Citron, Sudachi, Suyu and yuzu share the layout.
type yuzuFamily struct {
	name string // eden, citron, …: stored as the emulator
	nand string
	keys []string
}

func newYuzuFamily() Handler {
	for _, name := range []string{"eden", "citron", "sudachi", "suyu", "yuzu"} {
		custom := qtPath(expandHome("~/.config/"+name+"/qt-config.ini"), "Data%20Storage", "nand_directory")
		nand := firstExisting(
			custom,
			"~/.local/share/"+name+"/nand",
			"~/Emulation/storage/"+name+"/nand",
			"~/.var/app/org.yuzu_emu.yuzu/data/"+name+"/nand",
		)
		if nand == "" {
			continue
		}
		root := filepath.Dir(nand)
		return &yuzuFamily{name: name, nand: nand, keys: []string{
			filepath.Join(root, "keys", "prod.keys"),
			expandHome("~/.local/share/" + name + "/keys/prod.keys"),
			expandHome("~/.switch/prod.keys"),
		}}
	}
	return nil
}

func (y *yuzuFamily) ID() string { return y.name }

func (y *yuzuFamily) saveRoot() string {
	return filepath.Join(y.nand, "user", "save", "0000000000000000")
}

func (y *yuzuFamily) titleID(g Game) (string, error) {
	id, err := cachedID(g.Path, func() (uint64, error) { return SwitchTitleID(g.Path, y.keys) })
	if err != nil {
		return "", fmt.Errorf("reading the Switch title ID: %w", err)
	}
	return fmt.Sprintf("%016X", id), nil
}

func (y *yuzuFamily) Find(g Game) (LocalSave, bool, error) {
	tid, err := y.titleID(g)
	if err != nil {
		return LocalSave{}, false, err
	}
	m, _ := filepath.Glob(filepath.Join(y.saveRoot(), "*", tid))
	if p := newest(m); p != "" {
		return LocalSave{Emulator: y.ID(), Path: p, IsDir: true, FileName: tid + ".zip"}, true, nil
	}
	return LocalSave{}, false, nil
}

func (y *yuzuFamily) Target(g Game) (LocalSave, error) {
	tid, err := y.titleID(g)
	if err != nil {
		return LocalSave{}, err
	}
	// Use the profile folder: the most recently used non-zero user.
	user := strings.Repeat("0", 32)
	var users []string
	if m, _ := filepath.Glob(filepath.Join(y.saveRoot(), "*")); len(m) > 0 {
		for _, u := range m {
			if st, err := os.Stat(u); err == nil && st.IsDir() && filepath.Base(u) != user {
				users = append(users, u)
			}
		}
	}
	if u := newest(users); u != "" {
		user = filepath.Base(u)
	}
	return LocalSave{Emulator: y.ID(), Path: filepath.Join(y.saveRoot(), user, tid), IsDir: true, FileName: tid + ".zip"}, nil
}

// ── registry ─────────────────────────────────────────────────────────────────

// Detected picks handlers for installed emulators.
type Detected struct {
	once                                   sync.Once
	retroarch, melonds, azahar, ppsspp, yz Handler
}

func (d *Detected) detect() {
	d.retroarch = newRetroArch()
	d.melonds = newMelonDS()
	d.azahar = newAzahar()
	d.ppsspp = newPPSSPP()
	d.yz = newYuzuFamily()
}

// For returns the handler for a RomM platform slug, or nil if none of the
// installed emulators is supported for it.
func (d *Detected) For(platform string) Handler {
	d.once.Do(d.detect)
	pick := func(hs ...Handler) Handler {
		for _, h := range hs {
			if h != nil && !isNilHandler(h) {
				return h
			}
		}
		return nil
	}
	switch platform {
	case "3ds", "new-nintendo-3ds":
		return pick(d.azahar)
	case "switch":
		return pick(d.yz)
	case "psp":
		return pick(d.ppsspp)
	case "nds":
		return pick(d.melonds, d.retroarch)
	}
	if retroArchPlatforms[platform] {
		return pick(d.retroarch)
	}
	return nil
}

// isNilHandler catches typed nil pointers stored in the interface.
func isNilHandler(h Handler) bool {
	switch v := h.(type) {
	case *retroArch:
		return v == nil
	case *melonDS:
		return v == nil
	case *azahar:
		return v == nil
	case *ppsspp:
		return v == nil
	case *yuzuFamily:
		return v == nil
	}
	return false
}

// ID caches: reading headers of multi-GB ROMs once per run is plenty.
var (
	idCache   sync.Map // path -> uint64
	textCache sync.Map // path -> string
)

func cachedID(path string, read func() (uint64, error)) (uint64, error) {
	key := cacheKey(path)
	if v, ok := idCache.Load(key); ok {
		return v.(uint64), nil
	}
	id, err := read()
	if err == nil {
		idCache.Store(key, id)
	}
	return id, err
}

func cachedString(path string, read func() (string, error)) (string, error) {
	key := cacheKey(path)
	if v, ok := textCache.Load(key); ok {
		return v.(string), nil
	}
	s, err := read()
	if err == nil {
		textCache.Store(key, s)
	}
	return s, err
}

func cacheKey(path string) string {
	if st, err := os.Stat(path); err == nil {
		return path + "|" + strconv.FormatInt(st.ModTime().UnixNano(), 10)
	}
	return path
}
