package mutate

type Request struct {
	SourceFiles []string
	Count       int
}

type Mutator interface {
}
