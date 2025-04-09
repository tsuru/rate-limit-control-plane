# rate-limit-control-plane
A control plane to analyze and centralize rate limit among nginx instances

### Cleaning Dangling Images On Minikube

After running this project dozens of times, you may end up with a lot of dangling images. To clean them up, run the following command:

```bash
minikube ssh
docker rmi $(docker images --filter "dangling=true" -q)
```
