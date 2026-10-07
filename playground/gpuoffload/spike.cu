// gpuoffload spike: measures the two host-side costs any CUDA offload of
// the engine would pay, kernel launch overhead and PCIe transfer
// bandwidth, behind a plain C ABI so the Go driver loads the dll at
// runtime with no link-time CUDA dependency.
#include <cuda_runtime.h>
#include <cstddef>
#include <cstdint>

__global__ void noopKernel() {}

extern "C" {

// noop_launch fires iters empty kernel launches and returns the CUDA
// error code. The caller times the loop: per-launch cost is the floor any
// per-node GPU call would pay.
__declspec(dllexport) int32_t noop_launch(int32_t iters) {
    for (int32_t i = 0; i < iters; i++) {
        noopKernel<<<1, 32>>>();
    }
    return (int32_t)cudaGetLastError();
}

// copy_bandwidth_dgbps moves bytes host-to-device and back per iteration
// over pinned memory and returns the achieved combined bandwidth in
// deci-GB/s (GB/s x 10) as an integer, because a double return rides XMM0
// while the Go syscall trampoline reads the integer register. Negative on
// allocation failure.
__declspec(dllexport) int32_t copy_bandwidth_dgbps(size_t bytes, int32_t iters) {
    char* host = nullptr;
    char* dev = nullptr;
    if (cudaMallocHost(&host, bytes) != cudaSuccess) return -1;
    if (cudaMalloc(&dev, bytes) != cudaSuccess) { cudaFreeHost(host); return -2; }
    for (size_t i = 0; i < bytes; i++) host[i] = (char)i;
    cudaEvent_t start, stop;
    cudaEventCreate(&start);
    cudaEventCreate(&stop);
    cudaMemcpy(dev, host, bytes, cudaMemcpyHostToDevice);
    cudaDeviceSynchronize();
    cudaEventRecord(start);
    for (int32_t i = 0; i < iters; i++) {
        cudaMemcpy(dev, host, bytes, cudaMemcpyHostToDevice);
        cudaMemcpy(host, dev, bytes, cudaMemcpyDeviceToHost);
    }
    cudaEventRecord(stop);
    cudaEventSynchronize(stop);
    float ms = 0.0f;
    cudaEventElapsedTime(&ms, start, stop);
    cudaFree(dev);
    cudaFreeHost(host);
    cudaEventDestroy(start);
    cudaEventDestroy(stop);
    double seconds = ms / 1000.0;
    double moved = (double)bytes * (double)iters * 2.0;
    if (seconds <= 0.0) return -3;
    return (int32_t)(moved / seconds / 1e8);
}

}  // extern "C"
