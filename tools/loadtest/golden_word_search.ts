export default [
  { board: [["A","B","C","E"],["S","F","C","S"],["A","D","E","E"]], word: "ABCCED", result: true },
  { board: [["A","B","C","E"],["S","F","C","S"],["A","D","E","E"]], word: "SEE", result: true },
  { board: [["A","B","C","E"],["S","F","C","S"],["A","D","E","E"]], word: "ABCB", result: false },
  { board: [["A"]], word: "A", result: true },
  { board: [["A","B"],["C","D"]], word: "ABDC", result: true },
  { board: [["A","B"],["C","D"]], word: "DBCA", result: false },
  { board: [["A","B"],["C","D"]], word: "ABCD", result: false },
  { board: [["A","A","A","A"],["A","A","A","A"],["A","A","A","A"]], word: "AAAAAAAAAAAA", result: true },
  { board: [["A","A","A","A"],["A","A","A","A"],["A","A","A","A"]], word: "AAAAAAAAAAAAA", result: false },
  { board: [["A","B","C","D","E"],["F","G","H","I","J"],["K","L","M","N","O"],["P","Q","R","S","T"]], word: "ABCDIHNMLKFG", result: false },
] as const;
