package database

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/Waifu-im/waifu-api/constants"
	"github.com/Waifu-im/waifu-api/ipc"
	"github.com/Waifu-im/waifu-api/models"
	"github.com/lib/pq"
	"math/rand"
	"strconv"
	"time"
)

// Some methods to fetch data from the database

// FetchImages this method is charged of generating the appropriate query based on the user query params (after binding) and querying the database
func (database Database) FetchImages(
	isNsfw string,
	includedTags []string,
	excludedTags []string,
	includedFiles []string,
	excludedFiles []string,
	gif string,
	orderBy string,
	orientation string,
	limit int,
	full bool,
	width string,
	height string,
	byteSize string,
	userId int64,
) (ImageRows, time.Duration, error) {
	limitString := strconv.Itoa(limit)
	var parameters []any

	// Select some information about the images
	query := "SELECT DISTINCT Q.signature,Q.extension,Q.image_id,Q.favorites,Q.dominant_color,Q.source,Q.uploaded_at,Q.is_nsfw,Q.width,Q.height,Q.byte_size,"
	if userId != 0 {
		query += "Q.liked_at,"
	}
	// Select the tags and artist information from the "final"
	query += "Tags.id as tag_id,Tags.name as tag_name,Tags.description as tag_description,Tags.is_nsfw as tag_is_nsfw, " +
		"Artists.id as artist_id, Artists.name as artist_name, Artists.patreon as artist_patreon, Artists.pixiv as artist_pixiv, Artists.twitter as artist_twitter, Artists.deviant_art as artist_deviant_art  "
	// The sub query that will filter the images
	query += "FROM (" +
		"SELECT Images.signature,Images.extension,Images.image_id,Images.dominant_color,Images.source," +
		"Images.uploaded_at,Images.is_nsfw,Images.width,Images.height,Images.byte_size,Images.artist_id,"
	if userId != 0 {
		query += "FavImages.liked_at,"
	}
	query += "(SELECT COUNT(image_id) from FavImages WHERE image_id=Images.image_id) as favorites " +
		"FROM Images JOIN LinkedTags ON Images.image_id=LinkedTags.image_id JOIN Tags ON Tags.id=LinkedTags.tag_id "
	if userId != 0 {
		query += fmt.Sprintf("JOIN FavImages ON FavImages.image_id=Images.image_id AND FavImages.user_id=$%v ", len(parameters)+1)
		parameters = append(parameters, userId)
	}
	query += "WHERE not Images.under_review and not Images.hidden " + FormatNsfwTags(isNsfw, includedTags, &parameters)
	query += FormatGif(gif)
	query += FormatOrientation(orientation)
	query += FormatComparator("width", width)
	query += FormatComparator("height", height)
	query += FormatComparator("byte_size", byteSize)
	if len(includedFiles) > 0 {
		query += fmt.Sprintf("and (Images.image_id::text ILIKE ANY($%v) or Images.signature ILIKE ANY($%v))", len(parameters)+1, len(parameters)+1)
		parameters = append(parameters, pq.Array(includedFiles))
	}
	if len(excludedFiles) > 0 {
		query += fmt.Sprintf("and not (Images.image_id::text ILIKE ANY($%v) or Images.signature ILIKE ANY($%v)) ", len(parameters)+1, len(parameters)+1)
		parameters = append(parameters, pq.Array(excludedFiles))
	}
	if len(includedTags) > 0 {
		query += fmt.Sprintf("and Tags.name ILIKE ANY($%v) ", len(parameters)+1)
		parameters = append(parameters, pq.Array(includedTags))
	}
	if len(excludedTags) > 0 {
		query += fmt.Sprintf("and NOT EXISTS (SELECT 1 FROM LinkedTags AS lk JOIN Tags T ON lk.tag_id=T.id WHERE lk.image_id = Images.image_id AND T.name ILIKE ANY($%v)) ", len(parameters)+1)
		parameters = append(parameters, pq.Array(excludedTags))
	}
	query += "GROUP BY Images.image_id"
	if userId != 0 {
		query += ",FavImages.liked_at"
	}
	query += " "
	if len(includedTags) > 0 {
		query += "HAVING COUNT(*)=" + strconv.FormatInt(int64(len(includedTags)), 10) + " "
	}
	// If it's not in full mode or files has been provided, and it's not in fav mode and limit is provided then add the limit.
	query += FormatOrderBy(orderBy, "", true)
	if !(full || len(includedFiles) > 0) && userId == 0 && limit > 0 {
		query += "LIMIT " + limitString + " "
	} else if !(full || len(includedFiles) > 0) && userId == 0 {
		query += "LIMIT  1 "
	}
	query += ") AS Q JOIN LinkedTags ON LinkedTags.image_id=Q.image_id JOIN Tags ON Tags.id=LinkedTags.tag_id LEFT JOIN Artists ON Artists.id = Q.artist_id "
	query += FormatOrderBy(orderBy, "Q.", false)

	imageRows := ImageRows{database.configuration, []ImageRow{}}

	start := time.Now()

	rows, err := database.Db.Query(query, parameters...)
	if err != nil {
		return imageRows, time.Since(start), err
	}
	defer rows.Close()
	for rows.Next() {
		var imageRow ImageRow
		if userId == 0 {
			err = rows.Scan(
				&imageRow.Signature,
				&imageRow.Extension,
				&imageRow.ImageId,
				&imageRow.Favorites,
				&imageRow.DominantColor,
				&imageRow.Source,
				&imageRow.UploadedAt,
				&imageRow.IsNsfw,
				&imageRow.Width,
				&imageRow.Height,
				&imageRow.ByteSize,
				&imageRow.TagId,
				&imageRow.TagName,
				&imageRow.TagDescription,
				&imageRow.TagIsNsfw,
				&imageRow.ArtistId,
				&imageRow.ArtistName,
				&imageRow.ArtistPatreon,
				&imageRow.ArtistPixiv,
				&imageRow.ArtistTwitter,
				&imageRow.ArtistDeviantArt,
			)
		} else {
			err = rows.Scan(
				&imageRow.Signature,
				&imageRow.Extension,
				&imageRow.ImageId,
				&imageRow.Favorites,
				&imageRow.DominantColor,
				&imageRow.Source,
				&imageRow.UploadedAt,
				&imageRow.IsNsfw,
				&imageRow.Width,
				&imageRow.Height,
				&imageRow.ByteSize,
				&imageRow.LikedAt,
				&imageRow.TagId,
				&imageRow.TagName,
				&imageRow.TagDescription,
				&imageRow.TagIsNsfw,
				&imageRow.ArtistId,
				&imageRow.ArtistName,
				&imageRow.ArtistPatreon,
				&imageRow.ArtistPixiv,
				&imageRow.ArtistTwitter,
				&imageRow.ArtistDeviantArt,
			)
		}
		if err != nil {
			return imageRows, time.Since(start), err
		}
		imageRows.Rows = append(imageRows.Rows, imageRow)
	}
	if err = rows.Err(); err != nil {
		return imageRows, time.Since(start), err
	}
	if orderBy == constants.Random && len(imageRows.Rows) > 1 {
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(
			len(imageRows.Rows),
			func(i, j int) {
				imageRows.Rows[i], imageRows.Rows[j] = imageRows.Rows[j], imageRows.Rows[i]
			},
		)
	}
	return imageRows, time.Since(start), nil
}

func (database Database) ToggleImageInFav(userId int64, imageId int64) (string, error) {
	var empty int64
	if err := database.Db.QueryRow("SELECT image_id FROM FavImages WHERE user_id = $1 and image_id = $2", userId, imageId).Scan(&empty); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err = database.InsertImageToFav(userId, imageId); err != nil {
				return "", err
			}
			return "INSERTED", nil
		}
		return "", err
	}
	if err := database.DeleteImageFromFav(userId, imageId); err != nil {
		return "", err
	}
	return "DELETED", nil
}

func (database Database) InsertImageToFav(userId int64, imageId int64) error {
	_, err := database.Db.Exec("INSERT INTO FavImages(user_id,image_id) VALUES($1,$2)", userId, imageId)
	return err
}

func (database Database) DeleteImageFromFav(userId int64, imageId int64) error {
	var empty int64
	err := database.Db.QueryRow("DELETE FROM FavImages WHERE user_id=$1 and image_id=$2 RETURNING image_id", userId, imageId).Scan(&empty)
	return err
}

func (database Database) InsertUser(user ipc.User) error {
	_, err := database.Db.Exec("INSERT INTO Registered_user(id,name) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET name=$2", user.Id, user.FullName)
	return err
}

func (database Database) Report(userId int64, imageId int64, description *string) (ReportRes, error) {
	var res ReportRes
	err := database.Db.QueryRow("INSERT INTO Reported_images (author_id,description,image_id) VALUES ($1,$2,$3) ON CONFLICT(image_id) DO UPDATE SET author_id=excluded.author_id RETURNING author_id,description,image_id, (xmax!=0) as existed", userId, description, imageId).Scan(&res.AuthorId, &res.Description, &res.ImageId, &res.Existed)
	return res, err
}

func (database Database) GetTags() ([]models.Tag, error) {
	rows, err := database.Db.Query("SELECT id as tag_id, name, description, is_nsfw FROM Tags")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tagRows []models.Tag
	for rows.Next() {
		var tag models.Tag
		err := rows.Scan(&tag.TagId, &tag.Name, &tag.Description, &tag.IsNsfw)
		if err != nil {
			return tagRows, err
		}
		tagRows = append(tagRows, tag)
	}
	err = rows.Err()
	return tagRows, err
}

func (database Database) GetUserInformationFromToken(token string) (models.User, error) {
	user := models.User{}
	var isBlacklisted bool
	if err := database.Db.QueryRow(`SELECT id, name, token, is_blacklisted  FROM Registered_user WHERE token=$1`, token).Scan(&user.Id, &user.Name, &user.Token, &user.IsBlacklisted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user, nil
		}
		return user, err
	}
	if isBlacklisted {
		return user, constants.BlacklistedError
	}
	return user, nil
}

func (database Database) GetUserInformationFromId(userId int64) (models.User, error) {
	user := models.User{}
	var isBlacklisted bool
	if err := database.Db.QueryRow(`SELECT id, name, token, is_blacklisted  FROM Registered_user WHERE id=$1`, userId).Scan(&user.Id, &user.Name, &user.Token, &user.IsBlacklisted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user, nil
		}
		return user, err
	}
	if isBlacklisted {
		return user, constants.BlacklistedError
	}
	return user, nil
}

// GetMissingPermissions Provide a user id, an optional target user id, and permissions
// checks for all provided permissions that a user has the permissions globally or for the target if provided.
// Some might argue that performing a query for each permission might not be a good idea ,however fetching permissions
// position before might be a bit of a hassle and conduct to more query for the most command case : /fav , that only needs one permission.
// feel free to share your opinion if you have a better idea.
func (database Database) GetMissingPermissions(userId int64, targetUserId int64, permissions []string) ([]string, error) {
	var missing []string
	query := "SELECT user_permissions.user_id, user_permissions.target_id, permissions.position, permissions.name FROM user_permissions " +
		"JOIN permissions ON permissions.name=user_permissions.permission " +
		"JOIN registered_user on registered_user.id=user_permissions.user_id " +
		"WHERE registered_user.id=$1 " +
		"and (permissions.name=$2 or permissions.position > (SELECT position from permissions where name=$2))" +
		"and (user_permissions.target_id=$3 or user_permissions.target_id is NULL)"
	if targetUserId == userId && userId != 0 {
		return missing, nil
	}
	userPermissions := PermissionsInformation{}
	for _, perm := range permissions {
		if err := database.Db.QueryRow(query, userId, perm, targetUserId).Scan(
			&userPermissions.UserId,
			&userPermissions.TargetId,
			&userPermissions.Position,
			&userPermissions.Name,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				missing = append(missing, perm)
				continue
			}
			return missing, err
		}
	}
	return missing, nil
}

func (database Database) LogRequest(ip string, url string, userAgent string, userId int64, version string, execTime int64, headers string, responseBody string, statusCode int) {
	if _, err := database.Db.Exec(
		"INSERT INTO api_logs(remote_address,url,user_agent,user_id,version, query_exec_time, headers, response_body, status_code) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)",
		ip,
		url,
		CreateNullString(userAgent),
		CreateNullInt64(userId),
		CreateNullString(version),
		CreateNullInt64(execTime),
		headers,
		responseBody,
		statusCode,
	); err != nil {
		fmt.Println(err)
	}
}

func CreateNullString(s string) sql.NullString {
	if len(s) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}
func CreateNullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{
		Int64: i,
		Valid: true,
	}
}
