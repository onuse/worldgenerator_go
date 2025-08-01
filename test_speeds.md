# Speed Test Instructions

Run the simulation with `./worldgenerator` and open http://localhost:8080 in your browser.

## What to Look For:

1. **Console Output** - You should see:
   - `TIMING:` messages every second showing simulation performance
   - `SPEED CHANGE:` messages when you click speed buttons
   - `SLOW FRAME:` warnings if frames take >90ms

2. **Expected Performance**:
   - **Pause (0 yr/s)**: No simulation updates
   - **1000 yr/s**: ~100 years per frame, smooth updates
   - **10,000 yr/s**: ~1000 years per frame, visible erosion
   - **100,000 yr/s**: ~10,000 years per frame, rapid changes
   - **1M yr/s**: ~100,000 years per frame, geological timescales
   - **10M yr/s**: ~1M years per frame, major continental drift
   - **100M yr/s**: ~10M years per frame, complete reshaping

3. **Performance Metrics**:
   - SimTime: Time spent in simulation (<100ms is good)
   - Broadcast: Time to send to clients (<10ms is good)
   - Total frame time should be <100ms

## Troubleshooting:

If buttons don't work:
1. Check browser console for errors
2. Check server console for "SPEED CHANGE" messages
3. Ensure WebSocket is connected (check Network tab)

If simulation is too slow:
- Look for "SLOW FRAME" messages
- Check if GPU is being used (should see erosion happening)
- Try reducing mesh resolution in server.go (change level 5 to 4)