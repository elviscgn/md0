package md0

import "fmt"

const maxRenderedOutputBytes = 16 * 1024 * 1024

func RenderFragmentBounded(doc *Document, result *EvalResult) (string, error) {
	fragment, err := RenderFragment(doc, result)
	if err != nil {
		return "", err
	}
	if len(fragment) > maxRenderedOutputBytes {
		return "", fmt.Errorf("rendered document exceeds 16 MiB limit")
	}
	return fragment, nil
}

func RenderPatchesBounded(doc *Document, result *EvalResult, stats IncrementalStats) ([]DOMPatch, error) {
	patches, err := RenderPatches(doc, result, stats)
	if err != nil {
		return nil, err
	}
	total := 0
	for _, patch := range patches {
		if len(patch.HTML) > maxRenderedOutputBytes-total {
			return nil, fmt.Errorf("rendered patch response exceeds 16 MiB limit")
		}
		total += len(patch.HTML)
	}
	return patches, nil
}
