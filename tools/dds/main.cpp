#include <iostream>
#include "dll.h"

int main() {
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
