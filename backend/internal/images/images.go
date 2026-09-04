package images

type Image struct {
	ID          string `json:"id"`
	Fingerprint string `json:"fingerprint"`
	Alias       string `json:"alias"`
	Arch        string `json:"arch"`
}
