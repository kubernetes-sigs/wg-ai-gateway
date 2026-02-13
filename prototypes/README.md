## Prototypes code

`prototypes/` is a single go module. All prototypes code lives under `prototypes/` directory for code sharing and easy dependencies between different prototypes without code duplication.

### Adding a prototype
 
`prototypes/<name>/` e.g., `prototypes/payload-processor`

This should be the default option.

Do not add a new `go.mod` as all prototypes should share `prototypes/go.mod` (unless that is a must).

### Documentation

All prototypes must include a `README.md` at their root, e.g., `prototypes/payload-processor/README.md`.

It must describe:
- The purpose of the prototype / what it's exploring.
- How to build / run the prototype (basic demo script).
- any dependecies it has from other prototypes. 

It should describe:
- Any assumptions made.
- Open questions not addressed.
