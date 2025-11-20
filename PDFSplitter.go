package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/unidoc/unipdf/v3/model"
)

var scansPath = `F:\NFI\Printers\Canon 5540\Oscar`

// Extract quotation number from text
func extractNumber(text string) string {
	re := regexp.MustCompile(`Quotation No\.\s*:\s*(\w+)`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// Extract version from text
func extractVersion(text string) string {
	re := regexp.MustCompile(`Version\s*:\s*(\S+)`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		return match[1]
	}
	return "1"
}

// Run Tesseract OCR on a single image file
func ocrImage(imgPath string) (string, error) {
	cmd := exec.Command(`tesseract`, imgPath, "stdout")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Convert PDF page to PNG (requires pdftoppm)
func pdfPageToImage(pdfPath string, page int, outDir string) (string, error) {
	outPath := filepath.Join(outDir, fmt.Sprintf("page_%d.png", page))
	cmd := exec.Command("pdftoppm", "-f", fmt.Sprintf("%d", page+1), "-l", fmt.Sprintf("%d", page+1), "-png", pdfPath, filepath.Join(outDir, "page"))
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return outPath, nil
}

// Determine page type (CoA / Pilot / Unknown)
func pageType(text string) string {
	if strings.Contains(text, "Certificate of Analysis") || strings.Contains(text, "Specification Sheet") {
		return "CoA"
	} else if strings.Contains(text, "PILOT") {
		return "Pilot"
	}
	return "Unknown"
}

// Split a single PDF
func splitPDF(pdfPath string, wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Println("Processing:", pdfPath)

	// Load PDF
	reader, f, err := model.NewPdfReaderFromFile(pdfPath, nil)
	if err != nil {
		log.Println("Error opening PDF:", err)
	}
	defer f.Close()

	numPages, _ := reader.GetNumPages()
	fmt.Printf("PDF has %d pages\n", numPages)

	// Temporary directory for images
	tmpDir, _ := ioutil.TempDir("", "pdf_images")
	defer os.RemoveAll(tmpDir)

	texts := make([]string, numPages)

	// OCR each page
	for i := 0; i < numPages; i++ {
		imgPath, err := pdfPageToImage(pdfPath, i, tmpDir)
		if err != nil {
			log.Println("Error converting page to image:", err)
			return
		}
		text, err := ocrImage(imgPath)
		if err != nil {
			log.Println("OCR error:", err)
			return
		}
		texts[i] = text
	}

	// Detect CoA pages
	var hPages []int
	for i, t := range texts {
		if pageType(t) == "CoA" {
			hPages = append(hPages, i)
		}
	}

	// Detect Pilots (quote + version)
	pPages := make(map[[2]string][]int) // key: [quote, version]
	for i, t := range texts {
		number := extractNumber(t)
		if number != "" {
			version := extractVersion(t)
			key := [2]string{number, version}
			pPages[key] = append(pPages[key], i)
		}
	}

	// Output directories
	scriptDir, _ := os.Getwd()
	coaDir := filepath.Join(scriptDir, "splits")
	os.MkdirAll(coaDir, os.ModePerm)

	// Split CoA PDFs
	for i, start := range hPages {
		end := numPages
		if i+1 < len(hPages) {
			end = hPages[i+1]
		}
		writer := model.NewPdfWriter()
		for j := start; j < end; j++ {
			page, _ := reader.GetPage(j + 1)
			writer.AddPage(page)
		}
		outFile := filepath.Join(coaDir, fmt.Sprintf("CoA_%d.pdf", i+1))
		writer.WriteToFile(outFile)
	}

	// Split Pilot PDFs
	for key, pages := range pPages {
		number := key[0]
		version := key[1]
		outDir := filepath.Join(`F:\NFI\RID\Formulation\R&D Pilots\Pilots`, "QB-"+number)
		os.MkdirAll(outDir, os.ModePerm)

		outFile := filepath.Join(outDir, fmt.Sprintf("PilotReport_V%s.pdf", version))
		if _, err := os.Stat(outFile); !os.IsNotExist(err) {
			log.Fatalf("File already exists: %s", outFile)
		}

		writer := model.NewPdfWriter()
		for _, p := range pages {
			page, _ := reader.GetPage(p + 1)
			writer.AddPage(page)
		}
		writer.WriteToFile(outFile)
	}

	fmt.Println("Done:", pdfPath)
}

func main() {
	files, err := ioutil.ReadDir(scansPath)
	if err != nil {
		log.Fatal(err)
	}

	var pdfs []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".pdf") {
			pdfs = append(pdfs, filepath.Join(scansPath, f.Name()))
		}
	}

	var wg sync.WaitGroup
	for _, pdf := range pdfs {
		wg.Add(1)
		go splitPDF(pdf, &wg)
	}
	wg.Wait()
	fmt.Println("All PDFs processed.")
}
