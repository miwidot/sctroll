# Erzeugt das Programmsymbol aus Code, damit es reproduzierbar bleibt.
#
# Motiv: eine abgeschrägte Tastenkappe mit Blitz -- Zuschauer lösen per
# Tastendruck etwas aus. Die Formensprache (abgeschrägte Ecken, Cyan auf
# Dunkelblau, Salmon als Akzent) ist dieselbe wie in der Oberfläche.
#
# Gezeichnet wird vierfach vergrößert und dann heruntergerechnet; sonst
# fransen die schrägen Kanten in den kleinen Größen aus.
#
#   powershell -ExecutionPolicy Bypass -File build\make-icon.ps1

Add-Type -AssemblyName System.Drawing
$ErrorActionPreference = "Stop"

$outDir = Join-Path $PSScriptRoot ""
$sizes  = 16, 24, 32, 48, 64, 128, 256

# Farben aus der Oberfläche
$cSpace900 = [System.Drawing.Color]::FromArgb(255, 7, 16, 21)
$cSpace700 = [System.Drawing.Color]::FromArgb(255, 22, 50, 60)
$cData     = [System.Drawing.Color]::FromArgb(255, 124, 197, 209)
$cDataDim  = [System.Drawing.Color]::FromArgb(255, 49, 93, 101)
$cBrand    = [System.Drawing.Color]::FromArgb(255, 215, 92, 87)
$cBrandLo  = [System.Drawing.Color]::FromArgb(255, 168, 60, 56)

# Abgeschrägtes Rechteck: oben links und unten rechts eingeschnitten.
function New-ChamferPath([single]$x, [single]$y, [single]$w, [single]$h, [single]$cut) {
    $p = New-Object System.Drawing.Drawing2D.GraphicsPath
    $pts = @(
        (New-Object System.Drawing.PointF(($x + $cut), $y)),
        (New-Object System.Drawing.PointF(($x + $w), $y)),
        (New-Object System.Drawing.PointF(($x + $w), ($y + $h - $cut))),
        (New-Object System.Drawing.PointF(($x + $w - $cut), ($y + $h))),
        (New-Object System.Drawing.PointF($x, ($y + $h))),
        (New-Object System.Drawing.PointF($x, ($y + $cut)))
    )
    $p.AddPolygon($pts)
    $p
}

function New-BoltPath([single]$s, [single]$scale = 1.0) {
    # Kantiger Blitz, passend zur HUD-Anmutung -- keine runden Formen.
    # scale > 1 fuer kleine Symbolgroessen: dort muss die Form kraeftiger sein,
    # sonst verschwindet sie zwischen Rand und Fase.
    $coords = @(
        @(0.605, 0.130), @(0.315, 0.545), @(0.465, 0.545),
        @(0.395, 0.885), @(0.700, 0.440), @(0.545, 0.440)
    )
    $pts = foreach ($c in $coords) {
        $x = 0.5 + ($c[0] - 0.5) * $scale
        $y = 0.5 + ($c[1] - 0.5) * $scale
        New-Object System.Drawing.PointF(([single]($x * $s)), ([single]($y * $s)))
    }
    $p = New-Object System.Drawing.Drawing2D.GraphicsPath
    $p.AddPolygon([System.Drawing.PointF[]]$pts)
    $p
}

function New-Icon([int]$size, [bool]$detail) {
    $ss  = 4                      # Überabtastung
    $s   = $size * $ss
    $bmp = New-Object System.Drawing.Bitmap $s, $s
    $g   = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode     = 'AntiAlias'
    $g.InterpolationMode = 'HighQualityBicubic'
    $g.PixelOffsetMode   = 'HighQuality'
    $g.Clear([System.Drawing.Color]::Transparent)

    # Kleine Größen füllen die Kachel stärker aus -- bei 16 px ist jeder freie
    # Rand verschenkter Platz.
    $inset = [single]($s * $(if ($detail) { 0.070 } else { 0.035 }))
    $side  = [single]($s - 2 * $inset)
    $cut   = [single]($s * $(if ($detail) { 0.22 } else { 0.18 }))

    $cap = New-ChamferPath $inset $inset $side $side $cut

    # Schein hinter der Kappe -- gibt Tiefe, ohne dass eine Kante entsteht.
    if ($detail) {
        $glowPath = New-ChamferPath ([single]($inset * 0.35)) ([single]($inset * 0.35)) `
                                    ([single]($s - $inset * 0.70)) ([single]($s - $inset * 0.70)) `
                                    ([single]($cut * 1.15))
        $gb = New-Object System.Drawing.Drawing2D.PathGradientBrush($glowPath)
        $gb.CenterColor = [System.Drawing.Color]::FromArgb(70, 124, 197, 209)
        $gb.SurroundColors = @([System.Drawing.Color]::FromArgb(0, 124, 197, 209))
        $g.FillPath($gb, $glowPath)
        $gb.Dispose(); $glowPath.Dispose()
    }

    # Grundfläche: dunkel, damit der cyanfarbene Rand trägt
    $brush = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
        (New-Object System.Drawing.PointF(0, 0)),
        (New-Object System.Drawing.PointF([single]($s * 0.35), [single]$s)),
        $cSpace700, $cSpace900)
    $g.FillPath($brush, $cap)
    $brush.Dispose()

    # Sanfter Lichtverlauf von oben, auf die Kappe beschnitten. Kein eigenes
    # Rechteck mehr -- das hatte eine sichtbare Unterkante quer durchs Bild.
    if ($detail) {
        $state = $g.Save()
        $g.SetClip($cap)
        # Verlauf über die volle Höhe: endet er vorher, kachelt GDI+ ihn
        # dahinter und es entsteht eine sichtbare Querkante.
        $sheen = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
            (New-Object System.Drawing.PointF(0, 0)),
            (New-Object System.Drawing.PointF(0, [single]$s)),
            [System.Drawing.Color]::FromArgb(52, 150, 215, 226),
            [System.Drawing.Color]::FromArgb(0, 150, 215, 226))
        $g.FillRectangle($sheen, 0, 0, $s, $s)
        $sheen.Dispose()
        $g.Restore($state)
    }

    # Blitz
    $bolt = New-BoltPath ([single]$s) ([single]$(if ($detail) { 1.0 } else { 1.16 }))
    $bb = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
        (New-Object System.Drawing.PointF(0, 0)),
        (New-Object System.Drawing.PointF(0, [single]$s)),
        [System.Drawing.Color]::FromArgb(255, 232, 118, 112), $cBrandLo)
    $g.FillPath($bb, $bolt)
    $bb.Dispose()

    # Dunkle Kontur statt versetztem Schatten -- der wirkte schlampig und
    # verdoppelte die Silhouette.
    if ($detail) {
        $bp = New-Object System.Drawing.Pen(([System.Drawing.Color]::FromArgb(190, 12, 26, 32)), ([single]($s * 0.016)))
        $bp.LineJoin = 'Miter'
        $g.DrawPath($bp, $bolt)
        $bp.Dispose()
    }

    # Rand zuletzt: eine einzige klare Linie, kein doppelter Umriss
    $penW = [single]([Math]::Max($s * 0.030, $ss * 1.0))
    $pen = New-Object System.Drawing.Pen($cData, $penW)
    $pen.LineJoin = 'Miter'
    $pen.Alignment = 'Inset'
    $g.DrawPath($pen, $cap)
    $pen.Dispose()

    # Lichtkante auf der oberen Abschrägung: fängt das Licht wie eine echte
    # Fase und nimmt dem Rand das Aufgemalte.
    if ($detail) {
        $hp = New-Object System.Drawing.Pen(
            ([System.Drawing.Color]::FromArgb(215, 200, 240, 246)), ([single]($penW * 0.55)))
        $hp.StartCap = 'Round'; $hp.EndCap = 'Round'
        $g.DrawLine($hp,
            [single]($inset + $cut * 0.92), [single]($inset + $penW * 0.5),
            [single]($inset + $side - $penW), [single]($inset + $penW * 0.5))
        $g.DrawLine($hp,
            [single]($inset + $penW * 0.5), [single]($inset + $cut * 0.92),
            [single]($inset + $cut * 0.92), [single]($inset + $penW * 0.5))
        $hp.Dispose()
    }

    $cap.Dispose(); $bolt.Dispose(); $g.Dispose()

    # Herunterrechnen
    $final = New-Object System.Drawing.Bitmap $size, $size
    $fg = [System.Drawing.Graphics]::FromImage($final)
    $fg.InterpolationMode = 'HighQualityBicubic'
    $fg.PixelOffsetMode   = 'HighQuality'
    $fg.CompositingQuality = 'HighQuality'
    $fg.DrawImage($bmp, (New-Object System.Drawing.Rectangle(0, 0, $size, $size)))
    $fg.Dispose(); $bmp.Dispose()
    $final
}

# --- Einzelbilder ---
$pngs = @{}
foreach ($size in $sizes) {
    # Unter 32 px fallen Feinheiten weg, sie würden nur matschen.
    $img = New-Icon $size ($size -ge 32)
    $ms = New-Object System.IO.MemoryStream
    $img.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    $pngs[$size] = $ms.ToArray()
    $ms.Dispose(); $img.Dispose()
    "  {0,3}x{0,-3} {1,6} Bytes" -f $size, $pngs[$size].Length
}

# --- appicon.png für Wails ---
$big = New-Icon 512 $true
$big.Save((Join-Path $outDir "appicon.png"), [System.Drawing.Imaging.ImageFormat]::Png)
$big.Dispose()

# --- .ico zusammensetzen (PNG-Einträge, ab Windows Vista unterstützt) ---
$icoPath = Join-Path $outDir "windows\icon.ico"
$fs = [System.IO.File]::Create($icoPath)
$bw = New-Object System.IO.BinaryWriter($fs)

$bw.Write([UInt16]0)               # reserviert
$bw.Write([UInt16]1)               # Typ: Icon
$bw.Write([UInt16]$sizes.Count)

$offset = 6 + 16 * $sizes.Count
foreach ($size in $sizes) {
    $data = $pngs[$size]
    $bw.Write([Byte]$(if ($size -ge 256) { 0 } else { $size }))
    $bw.Write([Byte]$(if ($size -ge 256) { 0 } else { $size }))
    $bw.Write([Byte]0)             # Farbtabelle
    $bw.Write([Byte]0)             # reserviert
    $bw.Write([UInt16]1)           # Ebenen
    $bw.Write([UInt16]32)          # Bit pro Pixel
    $bw.Write([UInt32]$data.Length)
    $bw.Write([UInt32]$offset)
    $offset += $data.Length
}
foreach ($size in $sizes) { $bw.Write($pngs[$size]) }

$bw.Close(); $fs.Close()

"icon.ico    $((Get-Item $icoPath).Length) Bytes, $($sizes.Count) Größen"
"appicon.png $((Get-Item (Join-Path $outDir 'appicon.png')).Length) Bytes"
