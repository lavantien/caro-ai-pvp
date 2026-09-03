using Caro.Domain;
using Microsoft.Extensions.Configuration;
using Xunit;

namespace Caro.Api.Tests;

/// <summary>
/// ConfigurationBinder round-trips for the "Caro" section. Partial JSON
/// must leave every unlisted value at the compiled default.
/// </summary>
public class ConfigBindingTests
{
    private static CaroConfig Bind(params KeyValuePair<string, string?>[] pairs)
    {
        ConfigurationBuilder builder = new();
        builder.AddInMemoryCollection(pairs);
        return builder.Build().GetSection("Caro").Get<CaroConfig>()
            ?? throw new InvalidOperationException("section did not bind");
    }

    [Fact]
    public void EmptySectionBindsToDefaults()
    {
        CaroConfig bound = Bind(new KeyValuePair<string, string?>("Caro:Unused", "1"));
        CaroConfig expected = new();

        Assert.Equal(expected.MaxConcurrentGames, bound.MaxConcurrentGames);
        Assert.Equal(expected.AbandonedTimeoutMinutes, bound.AbandonedTimeoutMinutes);
        Assert.Equal(expected.DefaultTTSizeMB, bound.DefaultTTSizeMB);
        Assert.Equal(expected.DefaultSessionTTSizeMB, bound.DefaultSessionTTSizeMB);
        Assert.Equal(expected.OpeningSpreadRadius, bound.OpeningSpreadRadius);
        Assert.Equal(expected.TimeControl.Default, bound.TimeControl.Default);
        Assert.Equal(expected.TimeControl.Entries.Count, bound.TimeControl.Entries.Count);
        Assert.Equal(expected.DifficultyProfiles.Count, bound.DifficultyProfiles.Count);
        Assert.Equal("One", bound.DifficultyProfiles[1].ThreadsMode);
        Assert.Equal("L5", bound.DifficultyProfiles[5].ThreadsMode);
        bound.Validate();
    }

    [Fact]
    public void ScalarOverrideLeavesEverythingElseDefault()
    {
        CaroConfig bound = Bind(new KeyValuePair<string, string?>("Caro:MaxConcurrentGames", "8"));

        Assert.Equal(8, bound.MaxConcurrentGames);
        Assert.Equal(new CaroConfig().AbandonedTimeoutMinutes, bound.AbandonedTimeoutMinutes);
        Assert.Equal(new CaroConfig().OpeningSpreadRadius, bound.OpeningSpreadRadius);
        Assert.Equal(new CaroConfig().TimeControl.Default, bound.TimeControl.Default);
        bound.Validate();
    }

    [Fact]
    public void TimeControlOverrideReplacesEntry()
    {
        CaroConfig bound = Bind(
            new KeyValuePair<string, string?>("Caro:TimeControl:Entries:2+1:Canonical", "2+1"),
            new KeyValuePair<string, string?>("Caro:TimeControl:Entries:2+1:InitialTimeMs", "120000"),
            new KeyValuePair<string, string?>("Caro:TimeControl:Entries:2+1:IncrementSeconds", "1"));

        Constants.TimeControlData entry = bound.TimeControl.Entries["2+1"];
        Assert.Equal("2+1", entry.Canonical);
        Assert.Equal(120_000, entry.InitialTimeMs);
        Assert.Equal(1, entry.IncrementSeconds);

        // Untouched entries survive.
        Assert.True(bound.TimeControl.Entries["1+0"].InitialTimeMs == 60_000);
        Assert.Equal(new CaroConfig().TimeControl.Default, bound.TimeControl.Default);
        bound.Validate();
    }

    [Fact]
    public void ProfileOverrideMergesIntoSeededEntry()
    {
        CaroConfig bound = Bind(
            new KeyValuePair<string, string?>("Caro:DifficultyProfiles:1:TimeFraction", "0.10"),
            new KeyValuePair<string, string?>("Caro:DifficultyProfiles:1:MaxDepth", "3"));

        Assert.Equal(0.10, bound.DifficultyProfiles[1].TimeFraction);
        Assert.Equal(3, bound.DifficultyProfiles[1].MaxDepth);
        Assert.Equal("Novice", bound.DifficultyProfiles[1].Name);
        Assert.Equal(new CaroConfig().DifficultyProfiles[2].TimeFraction, bound.DifficultyProfiles[2].TimeFraction);
        bound.Validate();
    }

    [Fact]
    public void InvalidOverrideFailsValidation()
    {
        CaroConfig bound = Bind(new KeyValuePair<string, string?>("Caro:MaxConcurrentGames", "0"));
        Assert.Throws<InvalidOperationException>(bound.Validate);
    }
}
