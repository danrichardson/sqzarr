package transcoder

import (
	"bytes"
	"fmt"
	"os/exec"
)

// EncoderType identifies the hardware/software encoder to use.
type EncoderType string

const (
	EncoderVAAPI         EncoderType = "vaapi"
	EncoderVideoToolbox  EncoderType = "videotoolbox"
	EncoderNVENC         EncoderType = "nvenc"
	EncoderSoftware      EncoderType = "software"
)

// Encoder holds a detected encoder and the ffmpeg flags needed to invoke it.
type Encoder struct {
	Type        EncoderType
	DisplayName string
	// BuildArgs returns the full ffmpeg argument slice for encoding a file.
	// inputPath is the source, outputPath is the destination.
	BuildArgs func(inputPath, outputPath string) []string
}

// Detect probes the system and returns the best available encoder.
func Detect() (*Encoder, error) {
	if enc := probeVAAPI(); enc != nil {
		return enc, nil
	}
	if enc := probeVideoToolbox(); enc != nil {
		return enc, nil
	}
	if enc := probeNVENC(); enc != nil {
		return enc, nil
	}
	return softwareEncoder(), nil
}

// DetectAll probes every encoder independently and returns all that are
// available. The software encoder is always included as the last entry.
func DetectAll() []*Encoder {
	var encoders []*Encoder
	if enc := probeVAAPI(); enc != nil {
		encoders = append(encoders, enc)
	}
	if enc := probeVideoToolbox(); enc != nil {
		encoders = append(encoders, enc)
	}
	if enc := probeNVENC(); enc != nil {
		encoders = append(encoders, enc)
	}
	encoders = append(encoders, softwareEncoder())
	return encoders
}

// DetectByType returns the encoder for a specific type without probing.
// Returns nil if the type is unknown.
func DetectByType(t EncoderType) *Encoder {
	switch t {
	case EncoderVAAPI:
		return vaapiEncoder()
	case EncoderVideoToolbox:
		return videoToolboxEncoder()
	case EncoderNVENC:
		return nvencEncoder()
	case EncoderSoftware:
		return softwareEncoder()
	default:
		return nil
	}
}

func probeVAAPI() *Encoder {
	// Check that /dev/dri/renderD128 exists and hevc_vaapi is available.
	if err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Run(); err != nil {
		return nil
	}
	out, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output()
	if err != nil {
		return nil
	}
	// Require hevc_vaapi encoder in ffmpeg output.
	if !containsBytes(out, []byte("hevc_vaapi")) {
		return nil
	}
	// Quick device probe.
	probe := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-init_hw_device", "vaapi=va:/dev/dri/renderD128",
		"-f", "lavfi", "-i", "nullsrc=s=64x64:d=0.1",
		"-vf", "format=nv12,hwupload",
		"-c:v", "hevc_vaapi",
		"-frames:v", "1",
		"-f", "null", "-")
	if probe.Run() != nil {
		return nil
	}
	return vaapiEncoder()
}

func probeVideoToolbox() *Encoder {
	out, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output()
	if err != nil || !containsBytes(out, []byte("hevc_videotoolbox")) {
		return nil
	}
	return videoToolboxEncoder()
}

func probeNVENC() *Encoder {
	out, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output()
	if err != nil || !containsBytes(out, []byte("hevc_nvenc")) {
		return nil
	}
	return nvencEncoder()
}

func vaapiEncoder() *Encoder {
	return &Encoder{
		Type:        EncoderVAAPI,
		DisplayName: "Intel VAAPI (hevc_vaapi)",
		BuildArgs: func(inputPath, outputPath string) []string {
			return []string{
				"-y",
				"-init_hw_device", "vaapi=va:/dev/dri/renderD128",
				"-filter_hw_device", "va",
				"-i", inputPath,
				"-vf", `format=nv12,hwupload,scale_vaapi=w=min(1920\,iw):h=-2`,
				"-c:v", "hevc_vaapi",
				"-b:v", "2300k",
				"-maxrate", "4000k",
				"-bufsize", "4600k",
				"-c:a", "copy",
				"-c:s", "copy",
				"-max_muxing_queue_size", "9999",
				outputPath,
			}
		},
	}
}

func videoToolboxEncoder() *Encoder {
	return &Encoder{
		Type:        EncoderVideoToolbox,
		DisplayName: "Apple VideoToolbox (hevc_videotoolbox)",
		BuildArgs: func(inputPath, outputPath string) []string {
			return []string{
				"-y",
				"-i", inputPath,
				"-c:v", "hevc_videotoolbox",
				"-b:v", "2300k",
				"-maxrate", "4000k",
				"-bufsize", "4600k",
				"-c:a", "copy",
				"-c:s", "copy",
				"-max_muxing_queue_size", "9999",
				outputPath,
			}
		},
	}
}

func nvencEncoder() *Encoder {
	return &Encoder{
		Type:        EncoderNVENC,
		DisplayName: "NVIDIA NVENC (hevc_nvenc)",
		BuildArgs: func(inputPath, outputPath string) []string {
			return []string{
				"-y",
				"-hwaccel", "cuda",
				"-i", inputPath,
				"-c:v", "hevc_nvenc",
				"-preset", "p4",
				"-b:v", "2300k",
				"-maxrate", "4000k",
				"-bufsize", "4600k",
				"-c:a", "copy",
				"-c:s", "copy",
				"-max_muxing_queue_size", "9999",
				outputPath,
			}
		},
	}
}

func softwareEncoder() *Encoder {
	return &Encoder{
		Type:        EncoderSoftware,
		DisplayName: "Software (libx265)",
		BuildArgs: func(inputPath, outputPath string) []string {
			return []string{
				"-y",
				"-i", inputPath,
				"-c:v", "libx265",
				"-crf", "23",
				"-preset", "medium",
				"-b:v", "0",
				"-c:a", "copy",
				"-c:s", "copy",
				"-max_muxing_queue_size", "9999",
				outputPath,
			}
		},
	}
}

func (e *Encoder) String() string {
	return fmt.Sprintf("%s", e.DisplayName)
}

func containsBytes(haystack, needle []byte) bool {
	return bytes.Contains(haystack, needle)
}
