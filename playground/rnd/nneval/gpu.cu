#include <cuda_runtime.h>
#include <cstddef>
#include <cstdint>

__global__ void noopKernel() {}

#define NNSLOTS 2
#define MAXIN 256
#define MAXH1 128
#define MAXH2 64

struct slotState {
    float* w1; float* b1;
    float* w2; float* b2;
    float* w3; float* b3;
    int in, h1, h2;
    bool ready;
};

static slotState g[NNSLOTS];

__global__ void mlpKernel(const float* __restrict__ x,
                          const float* __restrict__ w1, const float* __restrict__ b1,
                          const float* __restrict__ w2, const float* __restrict__ b2,
                          const float* __restrict__ w3, const float* __restrict__ b3,
                          int in, int h1, int h2, float* __restrict__ out) {
    extern __shared__ float act[];
    float* h1buf = act;
    float* h2buf = act + h1;
    int t = threadIdx.x;
    for (int j = t; j < h1; j += blockDim.x) {
        float sum = b1[j];
        const float* row = w1 + (size_t)j * in;
        for (int i = 0; i < in; i++) sum += x[i] * row[i];
        h1buf[j] = sum > 0.0f ? sum : 0.0f;
    }
    __syncthreads();
    for (int j = t; j < h2; j += blockDim.x) {
        float sum = b2[j];
        const float* row = w2 + (size_t)j * h1;
        for (int i = 0; i < h1; i++) sum += h1buf[i] * row[i];
        h2buf[j] = sum > 0.0f ? sum : 0.0f;
    }
    __syncthreads();
    if (t == 0) {
        float sum = b3[0];
        const float* row = w3;
        for (int i = 0; i < h2; i++) sum += h2buf[i] * row[i];
        out[blockIdx.x] = sum;
    }
}

extern "C" {

__declspec(dllexport) int32_t noop_launch(int32_t iters) {
    for (int32_t i = 0; i < iters; i++) {
        noopKernel<<<1, 32>>>();
    }
    return (int32_t)cudaGetLastError();
}

__declspec(dllexport) int32_t mlp_setup(int32_t slot, int32_t inN, int32_t h1N, int32_t h2N,
                                        const float* w1, const float* b1,
                                        const float* w2, const float* b2,
                                        const float* w3, const float* b3) {
    if (slot < 0 || slot >= NNSLOTS) return -10;
    if (inN <= 0 || inN > MAXIN || h1N <= 0 || h1N > MAXH1 || h2N <= 0 || h2N > MAXH2) return -11;
    slotState& s = g[slot];
    if (!s.ready) {
        if (cudaMalloc(&s.w1, (size_t)MAXIN * MAXH1 * 4) != cudaSuccess) return -1;
        if (cudaMalloc(&s.b1, (size_t)MAXH1 * 4) != cudaSuccess) return -2;
        if (cudaMalloc(&s.w2, (size_t)MAXH1 * MAXH2 * 4) != cudaSuccess) return -3;
        if (cudaMalloc(&s.b2, (size_t)MAXH2 * 4) != cudaSuccess) return -4;
        if (cudaMalloc(&s.w3, (size_t)MAXH2 * 4) != cudaSuccess) return -5;
        if (cudaMalloc(&s.b3, 4) != cudaSuccess) return -6;
        s.ready = true;
    }
    s.in = inN; s.h1 = h1N; s.h2 = h2N;
    cudaError_t e;
    if ((e = cudaMemcpy(s.w1, w1, (size_t)inN * h1N * 4, cudaMemcpyHostToDevice)) != cudaSuccess) return (int32_t)e;
    if ((e = cudaMemcpy(s.b1, b1, (size_t)h1N * 4, cudaMemcpyHostToDevice)) != cudaSuccess) return (int32_t)e;
    if ((e = cudaMemcpy(s.w2, w2, (size_t)h1N * h2N * 4, cudaMemcpyHostToDevice)) != cudaSuccess) return (int32_t)e;
    if ((e = cudaMemcpy(s.b2, b2, (size_t)h2N * 4, cudaMemcpyHostToDevice)) != cudaSuccess) return (int32_t)e;
    if ((e = cudaMemcpy(s.w3, w3, (size_t)h2N * 4, cudaMemcpyHostToDevice)) != cudaSuccess) return (int32_t)e;
    if ((e = cudaMemcpy(s.b3, b3, 4, cudaMemcpyHostToDevice)) != cudaSuccess) return (int32_t)e;
    return 0;
}

__declspec(dllexport) int32_t mlp_check(int32_t slot, const float* input, float* output) {
    if (slot < 0 || slot >= NNSLOTS || !g[slot].ready) return -20;
    slotState& s = g[slot];
    float* dx = nullptr; float* dy = nullptr;
    if (cudaMalloc(&dx, (size_t)s.in * 4) != cudaSuccess) return -21;
    if (cudaMalloc(&dy, 4) != cudaSuccess) { cudaFree(dx); return -22; }
    cudaMemcpy(dx, input, (size_t)s.in * 4, cudaMemcpyHostToDevice);
    mlpKernel<<<1, 128, (s.h1 + s.h2) * sizeof(float)>>>(dx, s.w1, s.b1, s.w2, s.b2, s.w3, s.b3, s.in, s.h1, s.h2, dy);
    cudaError_t e = cudaDeviceSynchronize();
    if (e != cudaSuccess) { cudaFree(dx); cudaFree(dy); return (int32_t)e; }
    e = cudaMemcpy(output, dy, 4, cudaMemcpyDeviceToHost);
    cudaFree(dx); cudaFree(dy);
    return (int32_t)e;
}

__declspec(dllexport) int32_t mlp_us_per_iter(int32_t slot, int32_t batch, int32_t iters) {
    if (slot < 0 || slot >= NNSLOTS || !g[slot].ready) return -30;
    slotState& s = g[slot];
    if (batch <= 0 || iters <= 0) return -31;
    float* hx = nullptr; float* dx = nullptr; float* dy = nullptr; float* hy = nullptr;
    size_t inBytes = (size_t)batch * s.in * 4;
    if (cudaMallocHost(&hx, inBytes) != cudaSuccess) return -32;
    if (cudaMallocHost(&hy, (size_t)batch * 4) != cudaSuccess) { cudaFreeHost(hx); return -33; }
    if (cudaMalloc(&dx, inBytes) != cudaSuccess) { cudaFreeHost(hx); cudaFreeHost(hy); return -34; }
    if (cudaMalloc(&dy, (size_t)batch * 4) != cudaSuccess) { cudaFreeHost(hx); cudaFreeHost(hy); cudaFree(dx); return -35; }
    for (size_t i = 0; i < inBytes / 4; i++) hx[i] = (float)((int)(i * 2654435761u) % 1000 - 500) / 500.0f;
    int threads = 128;
    dim3 grid(batch);
    size_t shared = (size_t)(s.h1 + s.h2) * sizeof(float);
    cudaEvent_t t0, t1;
    cudaEventCreate(&t0); cudaEventCreate(&t1);
    cudaMemcpy(dx, hx, inBytes, cudaMemcpyHostToDevice);
    cudaDeviceSynchronize();
    cudaEventRecord(t0);
    for (int32_t i = 0; i < iters; i++) {
        cudaMemcpy(dx, hx, inBytes, cudaMemcpyHostToDevice);
        mlpKernel<<<grid, threads, shared>>>(dx, s.w1, s.b1, s.w2, s.b2, s.w3, s.b3, s.in, s.h1, s.h2, dy);
        cudaMemcpy(hy, dy, (size_t)batch * 4, cudaMemcpyDeviceToHost);
    }
    cudaEventRecord(t1);
    cudaEventSynchronize(t1);
    cudaError_t e = cudaGetLastError();
    float ms = 0.0f;
    cudaEventElapsedTime(&ms, t0, t1);
    cudaEventDestroy(t0); cudaEventDestroy(t1);
    cudaFreeHost(hx); cudaFreeHost(hy); cudaFree(dx); cudaFree(dy);
    if (e != cudaSuccess) return (int32_t)e;
    double us = (ms * 1000.0) / (double)iters;
    if (us < 0.0) return -36;
    return (int32_t)(us + 0.5);
}

__declspec(dllexport) int32_t copy_h2d_dgbps(size_t bytes, int32_t iters) {
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
    double moved = (double)bytes * (double)iters;
    if (seconds <= 0.0) return -3;
    return (int32_t)(moved / seconds / 1e8);
}

}
