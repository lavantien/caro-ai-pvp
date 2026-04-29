using System;
using System.Linq;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

var board = new Board();
board = board.PlaceStone(7, 7, Player.Red);
board = board.PlaceStone(8, 7, Player.Red);
board = board.PlaceStone(9, 7, Player.Red);

Console.WriteLine("Cells in iteration order:");
foreach (var cell in board.Cells.Where(c => !c.IsEmpty))
{
    Console.WriteLine($"  Cell(X={cell.X}, Y={cell.Y}), Player={cell.Player}");
}

var bitBoard = board.GetBitBoard(Player.Red);
Console.WriteLine($"BitBoard count: {bitBoard.CountBits()}");

// Check specific positions
Console.WriteLine($"GetBit(7, 7) = {bitBoard.GetBit(7, 7)}");
Console.WriteLine($"GetBit(8, 7) = {bitBoard.GetBit(8, 7)}");
Console.WriteLine($"GetBit(9, 7) = {bitBoard.GetBit(9, 7)}");

// Check the swapped positions
Console.WriteLine($"GetBit(7, 8) = {bitBoard.GetBit(7, 8)}");
Console.WriteLine($"GetBit(7, 9) = {bitBoard.GetBit(7, 9)}");

// Manual index calculation
Console.WriteLine("\nManual indices:");
Console.WriteLine($"index(7,7) = {7*19+7}");
Console.WriteLine($"index(8,7) = {7*19+8}");
Console.WriteLine($"index(9,7) = {7*19+9}");
Console.WriteLine($"index(7,8) = {8*19+7}");
Console.WriteLine($"index(7,9) = {9*19+7}");
