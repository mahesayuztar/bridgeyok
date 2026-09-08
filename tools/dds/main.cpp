#include <iostream>
#include "dll.h"

int solvePosition() {
    deal position{};
    int count;
    if (!(std::cin >> position.trump >> position.first >> count) || position.trump < 0 || position.trump > 4 || position.first < 0 || position.first > 3 || count < 0 || count > 3) return 2;
    for (int _index = 0; _index < count; ++_index) {
        if (!(std::cin >> position.currentTrickSuit[_index] >> position.currentTrickRank[_index])) return 2;
    }
    for (int _seatIndex = 0; _seatIndex < 4; ++_seatIndex)
        for (int _suitIndex = 0; _suitIndex < 4; ++_suitIndex)
            if (!(std::cin >> position.remainCards[_seatIndex][_suitIndex])) return 2;
    std::cin >> std::ws;
    if (!std::cin.eof()) return 2;
    SetResources(64, 1);
    futureTricks result{};
    if (SolveBoard(position, -1, 3, 1, &result, 0) != RETURN_NO_FAULT) return 3;
    std::cout << "[";
    bool first = true;
    for (int _index = 0; _index < result.cards; ++_index) {
        for (int rank = 2; rank <= 14; ++rank) {
            if (rank != result.rank[_index] && !(result.equals[_index] & (1 << rank))) continue;
            if (!first) std::cout << ',';
            first = false;
            std::cout << "{\"card\":{\"suit\":\"" << "SHDC"[result.suit[_index]]
                << "\",\"rank\":\"" << "23456789TJQKA"[rank - 2]
                << "\"},\"tricks\":" << result.score[_index] << '}';
        }
    }
    std::cout << "]" << std::endl;
    FreeMemory();
    return 0;
}

int main(int argc, char **argv) {
    if (argc == 2 && std::string(argv[1]) == "position") return solvePosition();
    int dealer, vulnerability;
    ddTableDeal deal{};
    if (!(std::cin >> dealer >> vulnerability) || dealer < 0 || dealer > 3 || vulnerability < 0 || vulnerability > 3) return 2;
    for (int _seatIndex = 0; _seatIndex < 4; ++_seatIndex) {
        for (int _suitIndex = 0; _suitIndex < 4; ++_suitIndex) {
            if (!(std::cin >> deal.cards[_seatIndex][_suitIndex]) || (deal.cards[_seatIndex][_suitIndex] & ~0x7ffcU)) return 2;
        }
    }
    std::cin >> std::ws;
    if (!std::cin.eof()) return 2;
    SetResources(64, 1);
    ddTableResults table{};
    parResultsMaster par{};
    if (CalcDDtable(deal, &table) != RETURN_NO_FAULT) { return 3; }
    if (DealerParBin(&table, &par, dealer, vulnerability) != RETURN_NO_FAULT) { return 4; }
    std::cout << "{\"solverVersion\":\"dds-2.9.0-8d75755\",\"table\":[";
    for (int _strainIndex = 0; _strainIndex < 5; ++_strainIndex) {
        if (_strainIndex) std::cout << ',';
        std::cout << '[';
        for (int _seatIndex = 0; _seatIndex < 4; ++_seatIndex) {
            if (_seatIndex) std::cout << ',';
            std::cout << table.resTable[_strainIndex][_seatIndex];
        }
        std::cout << ']';
    }
    std::cout << "],\"scoreNS\":" << par.score << ",\"contracts\":[";
    if (par.score != 0) {
        if (par.number < 1 || par.number > 10) { return 4; }
        for (int _contractIndex = 0; _contractIndex < par.number; ++_contractIndex) {
            const auto &contract = par.contracts[_contractIndex];
            if (_contractIndex) std::cout << ',';
            std::cout << "{\"level\":" << contract.level << ",\"denomination\":" << contract.denom
                << ",\"seats\":" << contract.seats << ",\"overTricks\":" << contract.overTricks
                << ",\"underTricks\":" << contract.underTricks << '}';
        }
    }
    std::cout << "]}" << std::endl;
    FreeMemory();
    return 0;
}
