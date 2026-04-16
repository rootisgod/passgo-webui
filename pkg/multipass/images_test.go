package multipass

import "testing"

func TestFindImages_RealCapture(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"find --format json": loadFixture(t, "find.json"),
	})
	c := NewClientWithRunner(discardLogger(), runner)
	images, err := c.FindImages()
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	// Partition by Type. The real capture has 4 images and 7 blueprints
	// (under the "blueprints (deprecated)" key).
	var imgCount, bpCount int
	for _, img := range images {
		switch img.Type {
		case "image":
			imgCount++
		case "blueprint":
			bpCount++
		default:
			t.Errorf("unknown type %q on %+v", img.Type, img)
		}
	}
	if imgCount != 4 {
		t.Errorf("image count: got %d, want 4", imgCount)
	}
	if bpCount != 7 {
		t.Errorf("blueprint count: got %d, want 7", bpCount)
	}

	// Images come before blueprints after sort.
	if images[0].Type != "image" {
		t.Errorf("first entry should be image, got %q", images[0].Type)
	}

	// Verify a specific image's aliases made it through.
	var found bool
	for _, img := range images {
		if img.Name == "24.04" && img.Type == "image" {
			found = true
			if len(img.Aliases) != 2 {
				t.Errorf("24.04 aliases: got %v", img.Aliases)
			}
			if img.Release != "24.04 LTS" {
				t.Errorf("24.04 release: %q", img.Release)
			}
		}
	}
	if !found {
		t.Error("24.04 image missing from results")
	}

	// Sort within group: images are alphabetical by name.
	var imageNames []string
	for _, img := range images {
		if img.Type == "image" {
			imageNames = append(imageNames, img.Name)
		}
	}
	for i := 1; i < len(imageNames); i++ {
		if imageNames[i-1] >= imageNames[i] {
			t.Errorf("images not sorted: %v", imageNames)
			break
		}
	}
}
