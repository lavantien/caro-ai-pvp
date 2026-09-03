using Caro.Domain;
using Xunit;

namespace Caro.Domain.Tests;

public class CaroConfigTests
{
    [Fact]
    public void DefaultsMatchConstants()
    {
        CaroConfig c = new();
        Assert.Equal(Constants.Limits.MaxConcurrentGames, c.MaxConcurrentGames);
        Assert.Equal(Constants.Limits.AbandonedTimeoutMinutes, c.AbandonedTimeoutMinutes);
        Assert.Equal(Constants.Transposition.DefaultSizeMB, c.DefaultTTSizeMB);
        Assert.Equal(Constants.Transposition.DefaultSessionSizeMB, c.DefaultSessionTTSizeMB);
        Assert.Equal(Constants.Opening.SpreadRadius, c.OpeningSpreadRadius);
    }

    [Fact]
    public void TimeControlDefaultsMatchConstants()
    {
        CaroConfig c = new();
        Assert.Equal(Constants.TimeControl.Default, c.TimeControl.Default);
        Assert.Equal(Constants.TimeControl.DefaultInitialTimeMs, c.TimeControl.DefaultInitialTimeMs);
        Assert.Equal(Constants.TimeControl.DefaultIncrementSeconds, c.TimeControl.DefaultIncrementSeconds);

        Assert.Equal(Constants.TimeControls.Count, c.TimeControl.Entries.Count);
        foreach ((string key, Constants.TimeControlData expected) in Constants.TimeControls)
        {
            Constants.TimeControlData actual = c.TimeControl.Entries[key];
            Assert.Equal(expected, actual);
        }
    }

    [Fact]
    public void DifficultyProfilesMatchConstants()
    {
        CaroConfig c = new();
        Assert.Equal(Constants.DifficultyProfiles.Count, c.DifficultyProfiles.Count);
        foreach (Constants.DifficultyProfileData expected in Constants.DifficultyProfiles)
        {
            DifficultyProfileOptions actual = c.DifficultyProfiles[expected.Level];
            Assert.Equal(expected.Level, actual.Level);
            Assert.Equal(expected.Name, actual.Name);
            Assert.Equal(expected.TimeFraction, actual.TimeFraction);
            Assert.Equal(expected.MaxDepth, actual.MaxDepth);
            Assert.Equal(expected.Threads.ToString(), actual.ThreadsMode);
            Assert.Equal(expected.UseVCF, actual.UseVCF);
            Assert.Equal(expected.VCFDepth, actual.VCFDepth);
            Assert.Equal(expected.Ponder, actual.Ponder);
            Assert.Equal(expected.TTSizeMB, actual.TTSizeMB);
        }
    }

    [Fact]
    public void DefaultPassesValidation()
    {
        CaroConfig.Default.Validate();
        new CaroConfig().Validate();
    }

    [Fact]
    public void DefaultAndNewInstanceAreEqual()
    {
        Assert.Equal(new CaroConfig().MaxConcurrentGames, CaroConfig.Default.MaxConcurrentGames);
        Assert.Equal(new CaroConfig().TimeControl.Default, CaroConfig.Default.TimeControl.Default);
        Assert.Equal(new CaroConfig().DifficultyProfiles.Count, CaroConfig.Default.DifficultyProfiles.Count);
    }

    [Theory]
    [InlineData(0)]
    [InlineData(-1)]
    public void RejectsInvalidMaxConcurrentGames(int value)
    {
        CaroConfig c = new() { MaxConcurrentGames = value };
        InvalidOperationException e = Assert.Throws<InvalidOperationException>(c.Validate);
        Assert.Contains("Caro:MaxConcurrentGames", e.Message);
    }

    [Fact]
    public void RejectsInvalidOpeningSpreadRadius()
    {
        CaroConfig c = new() { OpeningSpreadRadius = 9 };
        InvalidOperationException e = Assert.Throws<InvalidOperationException>(c.Validate);
        Assert.Contains("Caro:OpeningSpreadRadius", e.Message);
    }

    [Fact]
    public void RejectsUnknownDefaultTimeControl()
    {
        CaroConfig c = new() { TimeControl = new TimeControlOptions { Default = "" } };
        InvalidOperationException e = Assert.Throws<InvalidOperationException>(c.Validate);
        Assert.Contains("Caro:TimeControl:Default", e.Message);
    }

    [Fact]
    public void RejectsMissingDifficultyLevel()
    {
        CaroConfig c = new();
        c.DifficultyProfiles.Remove(1);
        InvalidOperationException e = Assert.Throws<InvalidOperationException>(c.Validate);
        Assert.Contains("Caro:DifficultyProfiles", e.Message);
    }

    [Fact]
    public void RejectsInvalidProfileField()
    {
        CaroConfig c = new();
        c.DifficultyProfiles[1].TimeFraction = 1.5;
        InvalidOperationException e = Assert.Throws<InvalidOperationException>(c.Validate);
        Assert.Contains("Level 1:TimeFraction", e.Message);
    }

    [Fact]
    public void RejectsInvalidThreadsMode()
    {
        CaroConfig c = new();
        c.DifficultyProfiles[3].ThreadsMode = "Seven";
        InvalidOperationException e = Assert.Throws<InvalidOperationException>(c.Validate);
        Assert.Contains("ThreadsMode", e.Message);
    }

    [Fact]
    public void RejectsOutOfRangeLevelKey()
    {
        CaroConfig c = new();
        c.DifficultyProfiles[6] = new DifficultyProfileOptions { Level = 6 };
        InvalidOperationException e = Assert.Throws<InvalidOperationException>(c.Validate);
        Assert.Contains("Caro:DifficultyProfiles:6", e.Message);
    }
}
