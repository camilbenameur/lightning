#include <bits/stdc++.h>
using namespace std;

int main() {
    vector<string> words{"world!", "Hello,"};
    ranges::reverse(words);
    cout << words[0] << ' ' << words[1] << endl;
}
