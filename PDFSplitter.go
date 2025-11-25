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

	"github.com/pdfcpu/pdfcpu/pkg/api"
    "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

var scansPath = `F:\NFI\Printers\Canon 5540\Oscar`
var basePilotDir = `F:\NFI\RID\Formulation\R&D Pilots\Pilots`


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
	cmd := exec.Command(`C:\Users\research2\Desktop\Python-3.13.9\Projects\PDFSplitter\tesseract-4.1.1\tesseract.exe`, imgPath, "stdout")
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tesseract failed: %w", err)
	}
	return string(out), nil
}

// Convert PDF page to PNG (requires pdftoppm)
func pdfPageToImage(pdfPath string, page int, outDir string) (string, error) {
	prefix := filepath.Join(outDir, "page")
	cmd := exec.Command(`C:\Users\research2\Desktop\Python-3.13.9\Projects\PDFSplitter\poppler\Library\bin\pdftoppm.exe`, "-f", fmt.Sprintf("%d", page+1), "-l", fmt.Sprintf("%d", page+1), "-png", pdfPath, prefix)
	
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
	    return "", fmt.Errorf("poppler failed: %w", err)
	}

	outPath := fmt.Sprintf("%s-%d.png", prefix, page+1)

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

	// Get number of pages using pdfcpu
	conf := model.NewDefaultConfiguration()
	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		log.Println("Error reading PDF:", err)
		return
	}
	numPages := ctx.PageCount
	fmt.Printf("PDF has %d pages\n", numPages)

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

	// Collect CoA pages
	var hPages []int
	for i, t := range texts {
		if pageType(t) == "CoA" {
			hPages = append(hPages, i)
		}
	}

	// Collect Pilot pages by [quote, version]
	pPages := make(map[[2]string][]int)
	for i, t := range texts {
		number := extractNumber(t)
		if number != "" {
			version := extractVersion(t)
			key := [2]string{number, version}
			pPages[key] = append(pPages[key], i)
		}
	}

	// Create output directories
	scriptDir, _ := os.Getwd()
	coaDir := filepath.Join(scriptDir, "splits")
	os.MkdirAll(coaDir, os.ModePerm)

	// Split CoA PDFs
	for i, start := range hPages {
		end := numPages
		if i+1 < len(hPages) {
			end = hPages[i+1]
		}
		pageNumbers := []string{}
		for p := start + 1; p <= end; p++ { // pdfcpu pages are 1-based
			pageNumbers = append(pageNumbers, fmt.Sprintf("%d", p))
		}
		outFile := filepath.Join(coaDir, fmt.Sprintf("CoA_%d.pdf", i+1))
		if err := api.ExtractPagesFile(pdfPath, outFile, pageNumbers, conf); err != nil {
			log.Println("CoA extract error:", err)
		}
	}

	// Split Pilot PDFs
	for key, pages := range pPages {
		number := key[0]
		version := key[1]
		outDir := filepath.Join(basePilotDir, "QB-"+number)
		os.MkdirAll(outDir, os.ModePerm)

		outFile := filepath.Join(outDir, fmt.Sprintf("PilotReport_V%s.pdf", version))
		if _, err := os.Stat(outFile); !os.IsNotExist(err) {
			log.Printf("File exists, skipping: %s", outFile)
			continue
		}

		pageNumbers := []string{}
		for _, p := range pages {
			pageNumbers = append(pageNumbers, fmt.Sprintf("%d", p+1))
		}

		if err := api.ExtractPagesFile(pdfPath, outFile, pageNumbers, conf); err != nil {
			log.Println("Pilot extract error:", err)
		}
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
	fmt.Scanln()
}