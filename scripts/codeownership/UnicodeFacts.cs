using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Text;
using System.Text.Json;

var output = new List<int[]>();
for (int c = 0; c <= 0x10ffff; c++)
{
    var surrogate = c >= 0xd800 && c <= 0xdfff;
    var upper = surrogate ? c : Rune.ToUpperInvariant(new Rune(c)).Value;
    var flags = 0;
    if (c <= 0xffff)
    {
        flags |= char.IsLetter((char)c) ? 1 : 0;
        flags |= char.IsDigit((char)c) ? 2 : 0;
        flags |= char.IsWhiteSpace((char)c) ? 4 : 0;
        flags |= CharUnicodeInfo.GetUnicodeCategory((char)c)
                     is UnicodeCategory.NonSpacingMark or UnicodeCategory.ConnectorPunctuation ? 8 : 0;
    }

    // All char classifications, plus supplementary invariant case mappings.
    if (c <= 0xffff || upper != c)
    {
        output.Add([c, upper, flags]);
    }
}

Console.WriteLine(JsonSerializer.Serialize(output));
