using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Newtonsoft.Json;

namespace AUOffsetManager
{
    /// <summary>
    /// Finds the class addresses of a build AUVC has no entry for, such as the Epic or
    /// Microsoft Store build of a version it knows from Steam. Each address is loaded by
    /// code that reads the same in every build of a version; Signatures.json holds that
    /// code with the addresses masked, written by scripts/signatures_from_build.py.
    /// The fields stay those of the template, so this only helps while the version is
    /// the same.
    /// </summary>
    public static class SignatureIndex
    {
        private const string Resource = "AUOffsetManager.Signatures.json";

        public sealed class Signature
        {
            [JsonProperty("sig")] public string Pattern { get; set; }
            [JsonProperty("displacement")] public int Displacement { get; set; }
        }

        public sealed class Set
        {
            [JsonProperty("template")] public string Template { get; set; }
            [JsonProperty("description")] public string Description { get; set; }
            [JsonProperty("classes")] public Dictionary<string, Signature> Classes { get; set; }
        }

        public static Dictionary<string, Set> LoadBundled()
        {
            using var stream = typeof(SignatureIndex).Assembly.GetManifestResourceStream(Resource);
            if (stream is null) return new Dictionary<string, Set>();
            using var reader = new StreamReader(stream);
            return JsonConvert.DeserializeObject<Dictionary<string, Set>>(reader.ReadToEnd()) ?? new Dictionary<string, Set>();
        }

        /// <summary>
        /// The address a pattern loads, relative to the start of the image, or null
        /// unless it occurs exactly once: a second hit could be any other class.
        /// </summary>
        public static int? Find(byte[] image, string pattern, int displacement)
        {
            var bytes = pattern.Split(' ', StringSplitOptions.RemoveEmptyEntries)
                .Select(token => token == "?" ? -1 : Convert.ToInt32(token, 16)).ToArray();
            // Search by a byte that is rare in code, not by the REX prefix every other instruction starts with.
            var common = new[] { 0x48, 0x4C, 0x8B, 0x89, 0x8D, 0x00, 0xFF };
            var first = Array.FindIndex(bytes, b => b >= 0 && !common.Contains(b));
            if (first < 0) first = Array.FindIndex(bytes, b => b >= 0);
            if (first < 0 || bytes.Length > image.Length) return null;

            int? found = null;
            var at = 0;
            while ((at = Array.IndexOf(image, (byte)bytes[first], at)) >= 0)
            {
                var start = at - first;
                at++;
                if (start < 0 || start + bytes.Length > image.Length) continue;
                var matches = true;
                for (var i = 0; i < bytes.Length && matches; i++)
                    matches = bytes[i] < 0 || image[start + i] == bytes[i];
                if (!matches) continue;
                if (found is not null) return null;
                found = start + displacement + 4 + BitConverter.ToInt32(image, start + displacement);
            }
            return found;
        }

        /// <summary>
        /// An entry for the build in image: the template's, with every class address
        /// found again. Null if a single one is not found once, or the template is gone.
        /// </summary>
        public static GameOffsets Derive(byte[] image, IReadOnlyDictionary<string, GameOffsets> index, Set set)
        {
            if (set?.Classes is null || set.Template is null || !index.TryGetValue(set.Template, out var template)) return null;

            var moved = new Dictionary<int, int>();
            foreach (var (name, signature) in set.Classes)
            {
                var property = typeof(GameOffsets).GetProperty(name);
                var address = Find(image, signature.Pattern, signature.Displacement);
                if (property is null || address is null) return null;
                moved[(int)property.GetValue(template)!] = address.Value;
            }

            var derived = JsonConvert.DeserializeObject<GameOffsets>(JsonConvert.SerializeObject(template));
            foreach (var name in set.Classes.Keys)
            {
                var property = typeof(GameOffsets).GetProperty(name)!;
                property.SetValue(derived, moved[(int)property.GetValue(derived)!]);
            }
            foreach (var property in typeof(GameOffsets).GetProperties().Where(p => p.PropertyType == typeof(int[])))
            {
                if (property.GetValue(derived) is int[] { Length: > 0 } chain && moved.TryGetValue(chain[0], out var address))
                    chain[0] = address;
            }
            derived.Description = template.Description + " – found by signature";
            return derived;
        }
    }
}
