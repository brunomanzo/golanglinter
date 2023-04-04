package linter

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

type StructCode struct {
	Name      string
	Node      ast.Node
	Functions []FunctionCode
}

type FunctionCode struct {
	Name                string
	Node                ast.Node
	ReceiverTypeName    string
	UnnecessaryNilCheck bool
	Params              []ParamsFunctionCode
	LogCall             *LogCall
}

type LogCall struct {
	Node *ast.Node
}

type ParamsFunctionCode struct {
	Name string
	Type string
}

func (f *ParamsFunctionCode) singleParamNameMatchs() bool {
	firstLetter1 := f.Name[:1]
	firstLetter2 := f.Type[:1]

	return firstLetter1 == strings.ToLower(firstLetter2)
}

func (f *FunctionCode) isPublic() bool {
	return unicode.IsUpper([]rune(f.Name)[0])
}

func (f *FunctionCode) receiverIsCorret(structName string) bool {
	firstLetter1 := f.ReceiverTypeName[:1]
	firstLetter2 := structName[:1]

	return firstLetter1 == strings.ToLower(firstLetter2)
}

//nolint:gochecknoglobals
var flagSet flag.FlagSet

func NewAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:  "dev-prod",
		Doc:   "dev-prod rules",
		Run:   run,
		Flags: flagSet,
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	structs := make([]StructCode, 0)
	for _, f := range pass.Files {
		fmt.Println(f.Name)
		ast.Inspect(f, func(node ast.Node) bool {
			// your code goes here

			// Find all struct declarations
			genDecl, ok := node.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				return true
			}

			// Find struct declarations
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Type == nil {
					continue
				}

				// // Check if this is the struct we're looking for
				_, _ = typeSpec.Type.(*ast.StructType)
				structCode := StructCode{
					Name:      typeSpec.Name.Name,
					Node:      node,
					Functions: make([]FunctionCode, 0),
				}

				ast.Inspect(f, func(nodeInner ast.Node) bool {
					if funcDecl, ok := nodeInner.(*ast.FuncDecl); ok {
						// Check if the function has a receiver
						if funcDecl.Recv != nil {
							// Get the type of the receiver
							var receiverType ast.Expr
							if len(funcDecl.Recv.List) > 0 {
								switch t := funcDecl.Recv.List[0].Type.(type) {
								case *ast.Ident:
									receiverType = t
								case *ast.StarExpr:
									receiverType = t.X
								}
								// Check if the receiver type is the same as the struct type
								if id, ok := receiverType.(*ast.Ident); ok && id.Name == typeSpec.Name.Name {
									receiverName := funcDecl.Recv.List[0].Names[0].Name

									var params []ParamsFunctionCode
									for _, param := range funcDecl.Type.Params.List {
										name := ""
										if len(param.Names) > 0 {
											name = param.Names[0].Name
										}
										if ident, ok := param.Type.(*ast.Ident); ok {
											params = append(params, ParamsFunctionCode{
												Name: name,
												Type: ident.Name,
											})
										}
									}

									functionCode := FunctionCode{
										Name:                funcDecl.Name.Name,
										Node:                nodeInner,
										ReceiverTypeName:    receiverName,
										Params:              params,
										UnnecessaryNilCheck: false,
									}

									logNode := hasFmtOrLogCall(funcDecl)

									if logNode != nil {
										functionCode.LogCall = &LogCall{
											Node: logNode,
										}
									}

									if funcDecl.Body != nil && len(funcDecl.Body.List) > 1 {
										lastStmt := funcDecl.Body.List[len(funcDecl.Body.List)-1]
										if retStmt, ok := lastStmt.(*ast.ReturnStmt); ok {
											if len(retStmt.Results) == 1 {
												if ident, ok := retStmt.Results[0].(*ast.Ident); ok {
													if ident.Name == "nil" {
														before := funcDecl.Body.List[len(funcDecl.Body.List)-2]
														if ifStmt, okBefore := before.(*ast.IfStmt); okBefore {
															if len(ifStmt.Body.List) > 0 {

																returnStmt := ifStmt.Body.List[len(ifStmt.Body.List)-1]
																if retStmt, ok := returnStmt.(*ast.ReturnStmt); ok {
																	if retStmtIdent, b := retStmt.Results[0].(*ast.Ident); b {
																		if retStmtIdent.Name == "err" {
																			functionCode.UnnecessaryNilCheck = true
																		}
																	}

																}
															}
														}
													}
												}
											}
										}
									}

									structCode.Functions = append(structCode.Functions, functionCode)
								}
							}
						}
					}
					return true
				})

				structs = append(structs, structCode)
			}

			return true
		})
	}

	for _, structCode := range structs {
		var singlePublic bool = true
		for _, functionCode := range structCode.Functions {
			if singlePublic && functionCode.isPublic() {
				singlePublic = false
			} else if functionCode.isPublic() {
				pass.Reportf(functionCode.Node.Pos(), "More than 1 public function")
			}

			if !functionCode.receiverIsCorret(structCode.Name) {
				pass.Reportf(functionCode.Node.Pos(), "Receiver name does not comply with single letter rule")
			}

			if functionCode.UnnecessaryNilCheck {
				pass.Reportf(functionCode.Node.Pos(), "Function have unnecessary nil check at end")
			}

			if functionCode.LogCall != nil {
				pass.Reportf((*functionCode.LogCall.Node).Pos(), "Don't use `log` package to log, use loggrus")
			}

			for _, param := range functionCode.Params {
				if len(param.Name) == 1 && !param.singleParamNameMatchs() {
					pass.Reportf(functionCode.Node.Pos(), "Single character parameter name does not match with type")
				}
			}
		}
	}
	return nil, nil
}

func hasFmtOrLogCall(f *ast.FuncDecl) *ast.Node {
	var hasCall *ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := fun.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == "log" {
			hasCall = &n
		}
		return true
	})
	return hasCall
}
