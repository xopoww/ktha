export default [
  { coins: [1, 2, 5], amount: 11, result: 3 },
  { coins: [2], amount: 3, result: -1 },
  { coins: [1], amount: 0, result: 0 },
  { coins: [1, 5, 10, 25], amount: 100, result: 4 },
  { coins: [1, 5, 10, 25], amount: 999, result: 45 },
  { coins: [3, 7], amount: 1, result: -1 },
  { coins: [1, 3, 4], amount: 6, result: 2 },
  { coins: [2, 5, 10, 1], amount: 27, result: 4 },
  { coins: [1, 5, 10, 25], amount: 10000, result: 400 },
  { coins: [1, 5, 10, 25], amount: 99999, result: 4005 },
] as const;
