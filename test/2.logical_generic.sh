#!/bin/bash
set -euxo pipefail

TAG="${TAG:-"master"}"
IMAGE2TEST="registry.gitlab.com/postgres-ai/database-lab/dblab-server:${TAG}"
SOURCE_DBNAME="${SOURCE_DBNAME:-test}"
SOURCE_HOST="${SOURCE_HOST:-172.17.0.1}"
SOURCE_PORT="${SOURCE_PORT:-7432}"
SOURCE_USERNAME="${SOURCE_USERNAME:-postgres}"
SOURCE_PASSWORD="${SOURCE_PASSWORD:-secretpassword}"
POSTGRES_VERSION="${POSTGRES_VERSION:-13}"

DIR=${0%/*}

if [[ "${SOURCE_HOST}" = "172.17.0.1" ]]; then
### Step 0. Create source database
  sudo rm -rf "$(pwd)"/postgresql/"${POSTGRES_VERSION}"/test || true
  sudo docker run \
    --name postgres"${POSTGRES_VERSION}" \
    --label pgdb \
    --privileged \
    --publish 172.17.0.1:"${SOURCE_PORT}":5432 \
    --env PGDATA=/var/lib/postgresql/pgdata \
    --env POSTGRES_USER="${SOURCE_USERNAME}" \
    --env POSTGRES_PASSWORD="${SOURCE_PASSWORD}" \
    --env POSTGRES_DB="${SOURCE_DBNAME}" \
    --env POSTGRES_HOST_AUTH_METHOD=md5 \
    --volume "$(pwd)"/postgresql/"${POSTGRES_VERSION}"/test:/var/lib/postgresql/pgdata \
    --detach \
    postgres:"${POSTGRES_VERSION}-alpine"

  for i in {1..300}; do
    sudo docker exec -it postgres"${POSTGRES_VERSION}" psql -d "${SOURCE_DBNAME}" -U postgres -c 'select' > /dev/null 2>&1  && break || echo "test database is not ready yet"
    sleep 1
  done

  # Generate data in the test database using pgbench
  # 1,000,000 accounts, ~0.14 GiB of data.
  sudo docker exec -it postgres"${POSTGRES_VERSION}" pgbench -U postgres -i -s 10 "${SOURCE_DBNAME}"

  # Database info
  sudo docker exec -it postgres"${POSTGRES_VERSION}" psql -U postgres -c "\l+ ${SOURCE_DBNAME}"
fi

### Step 1. Prepare a machine with disk, Docker, and ZFS
source "${DIR}/_prerequisites.ubuntu.sh"
source "${DIR}/_zfs.file.sh"


### Step 2. Configure and launch the Database Lab Engine

# Copy the contents of configuration example 
mkdir -p ~/.dblab

curl https://gitlab.com/postgres-ai/database-lab/-/raw/"${TAG}"/configs/config.example.logical_generic.yml \
 --output ~/.dblab/server.yml

# Edit the following options
sed -ri "s/^(\s*)(debug:.*$)/\1debug: true/" ~/.dblab/server.yml
sed -ri "s/^(\s*)(dbname:.*$)/\1dbname: ${SOURCE_DBNAME}/" ~/.dblab/server.yml
sed -ri "s/^(\s*)(host: 34.56.78.90$)/\1host: ${SOURCE_HOST}/" ~/.dblab/server.yml
sed -ri "s/^(\s*)(port: 5432$)/\1port: ${SOURCE_PORT}/" ~/.dblab/server.yml
sed -ri "s/^(\s*)(username: postgres$)/\1username: ${SOURCE_USERNAME}/" ~/.dblab/server.yml
sed -ri "s/^(\s*)(password:.*$)/\1password: ${SOURCE_PASSWORD}/" ~/.dblab/server.yml
# replace postgres version
sed -ri "s/:13/:${POSTGRES_VERSION}/g"  ~/.dblab/server.yml

## Launch Database Lab server
sudo docker run \
  --name dblab_server \
  --label dblab_control \
  --privileged \
  --publish 2345:2345 \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume /var/lib/dblab/dblab_pool/dump:/var/lib/dblab/dblab_pool/dump \
  --volume /var/lib/dblab:/var/lib/dblab/:rshared \
  --volume ~/.dblab/server.yml:/home/dblab/configs/config.yml \
  --env DOCKER_API_VERSION=1.39 \
  --detach \
  "${IMAGE2TEST}"

# Check the Database Lab Engine logs
sudo docker logs dblab_server -f 2>&1 | awk '{print "[CONTAINER dblab_server]: "$0}' &

### Waiting for the Database Lab Engine initialization.
for i in {1..300}; do
  curl http://localhost:2345 > /dev/null 2>&1 && break || echo "dblab is not ready yet"
  sleep 1
done


### Step 3. Start cloning

# Install Database Lab client CLI
curl https://gitlab.com/postgres-ai/database-lab/-/raw/master/scripts/cli_install.sh | bash
sudo mv ~/.dblab/dblab /usr/local/bin/dblab

dblab --version

# Initialize CLI configuration
dblab init \
  --environment-id=test \
  --url=http://localhost:2345 \
  --token=secret_token \
  --insecure

# Check the configuration by fetching the status of the instance:
dblab instance status


## Create a clone
dblab clone create \
  --username dblab_user_1 \
  --password secret_password \
  --id testclone

# Connect to a clone and check the available table
PGPASSWORD=secret_password psql \
  "host=localhost port=6000 user=dblab_user_1 dbname=${SOURCE_DBNAME}" -c '\dt+'

# Drop table
PGPASSWORD=secret_password psql \
  "host=localhost port=6000 user=dblab_user_1 dbname=${SOURCE_DBNAME}" -c 'drop table pgbench_accounts'

PGPASSWORD=secret_password psql \
  "host=localhost port=6000 user=dblab_user_1 dbname=${SOURCE_DBNAME}" -c '\dt+'

## Reset clone
dblab clone reset testclone

# Check the status of the clone
dblab clone status testclone

# Check the database objects (everything should be the same as when we started)
PGPASSWORD=secret_password psql \
  "host=localhost port=6000 user=dblab_user_1 dbname=${SOURCE_DBNAME}" -c '\dt+'

### Step 4. Destroy clone
dblab clone destroy testclone
dblab clone list

### Finish. clean up
source "${DIR}/_cleanup.sh"
